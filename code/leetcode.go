package code

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/ahmed-e-abdulaziz/glsync/config"
)

//go:embed leetcode-graphql/submission-details-query.json
var submissionDetailsQuery string

//go:embed leetcode-graphql/submission-list-query.json
var submissionListQuery string

//go:embed leetcode-graphql/user-progress-question-list-query.json
var userProgressQuestionListQuery string

//go:embed leetcode-graphql/question-details-query.json
var questionDetailsQuery string

const (
	maxRetry                 = 25              // LeetCode API can fail A LOT :( It requires a ton of retries when it fails
	backoffTime              = 1 * time.Second // 1 second to avoid keep using LeetCode API when it fails
	acceptedSubmissionStatus = 10
	submissionListPageSize   = 20
)

// Implementation of CodeClient for LeetCode
type leetcode struct {
	cfg        config.Config
	graphqlUrl string
}

func NewLeetCode(cfg config.Config, leetcodeGraphqlUrl string) leetcode {
	return leetcode{cfg, leetcodeGraphqlUrl}
}

// Fetches submissions from LeetCode
//
// Requires cfg.LcCookie to be set correctly or will fail due to access errors
// Returns an array of [Submission] struct
func (lc leetcode) FetchSubmissions() ([]Submission, error) {
	log.Println("\n==============\nFetching submissions next")
	questions, err := lc.fetchQuestions()
	if err != nil {
		log.Printf("Error fetching questions: %v\n", err)
		return nil, errors.New("failed to fetch questions from LeetCode")
	}

	log.Printf("User has %v solved questions on LeetCode, fetching accepted submissions for each next\n", len(questions))
	submissions := make([]Submission, 0, len(questions))

	for _, question := range questions {
		log.Printf("\tFetching accepted submissions for question: %v %v\n", question.FrontendId, question.Title)
		questionSubmissions, err := lc.fetchQuestionSubmissions(question)
		if err != nil {
			log.Printf("Warning: Failed to fetch accepted submissions for question %s: %v\n", question.Title, err)
			continue
		}
		submissions = append(submissions, questionSubmissions...)
	}

	if len(submissions) == 0 {
		return nil, errors.New("failed to fetch any submissions successfully")
	}

	log.Printf("Fetched %d accepted submissions successfully across %d solved questions\n==============\n", len(submissions), len(questions))
	return submissions, nil
}

func (lc leetcode) fetchQuestionSubmissions(question lcQuestion) ([]Submission, error) {
	details, err := lc.fetchQuestionDetails(question.TitleSlug)
	if err != nil {
		log.Printf("Warning: Failed to fetch metadata for question %s: %v\n", question.Title, err)
	}

	lcSubmissions, err := lc.fetchSubmissionOverviews(question.TitleSlug)
	if err != nil {
		log.Printf("Error fetching question submissions: %v\n", err)
		return nil, errors.New("submission overview error")
	}

	if len(lcSubmissions) == 0 {
		return nil, nil
	}

	tags := extractTagNames(details.TopicTags)
	submissions := make([]Submission, 0, len(lcSubmissions))
	for _, lcSubmission := range lcSubmissions {
		code, err := lc.fetchSubmissionCode(lcSubmission.Id, 0)
		if err != nil {
			log.Printf("Warning: Error fetching submission code for submission %s: %v\n", lcSubmission.Id, err)
			continue
		}

		timestamp, err := parseSubmissionTimestamp(lcSubmission.Timestamp)
		if err != nil {
			log.Printf("Warning: Error parsing submission timestamp for submission %s: %v\n", lcSubmission.Id, err)
			continue
		}

		submissions = append(submissions, Submission{
			Id:              question.FrontendId,
			Title:           question.Title,
			TitleSlug:       question.TitleSlug,
			SubmissionId:    lcSubmission.Id,
			LastSubmittedAt: timestamp,
			Lang:            lcSubmission.Lang,
			Code:            code,
			Difficulty:      details.Difficulty,
			Tags:            tags,
		})
	}

	if len(submissions) == 0 {
		return nil, errors.New("submission code error")
	}

	return submissions, nil
}

// Fetches question to extract required info for Submission struct
// Uses LC's GraphQl query that's called userProgressQuestionList
func (lc leetcode) fetchQuestions() ([]lcQuestion, error) {
	bodyBytes, err := lc.queryLeetcode(userProgressQuestionListQuery)
	if err != nil {
		log.Println(err)
		return nil, fmt.Errorf("error fetching user questions from leetcode: %w", err)
	}
	body := &RequestBody[lcUserProgressQuestionListData]{}
	err = json.Unmarshal(bodyBytes, body)
	if err != nil {
		log.Println(err)
		return nil, fmt.Errorf("error parsing user questions response from leetcode: %w", err)
	}
	return body.Data.QuestionsList.Questions, nil
}

func (lc leetcode) fetchQuestionDetails(titleSlug string) (lcQuestionDetails, error) {
	bodyBytes, err := lc.queryLeetcode(fmt.Sprintf(questionDetailsQuery, titleSlug))
	if err != nil {
		return lcQuestionDetails{}, fmt.Errorf("error fetching question details from leetcode: %w", err)
	}

	body := &RequestBody[lcQuestionDetailsData]{}
	if err := json.Unmarshal(bodyBytes, body); err != nil {
		return lcQuestionDetails{}, fmt.Errorf("error parsing question details response from leetcode: %w", err)
	}
	return body.Data.Question, nil
}

// Fetches accepted submission overviews for a question.
// Uses LC's GraphQl query that's called submissionList.
func (lc leetcode) fetchSubmissionOverviews(titleSlug string) ([]lcSumbissionOverview, error) {
	var (
		allSubmissions []lcSumbissionOverview
		lastKey        *string
	)

	for {
		query := buildSubmissionListQuery(titleSlug, lastKey, submissionListPageSize, acceptedSubmissionStatus)
		bodyBytes, err := lc.queryLeetcode(query)
		if err != nil {
			return nil, fmt.Errorf("error fetching submission overview from leetcode: %w", err)
		}

		body := &RequestBody[lcSubmissionListData]{}
		if err := json.Unmarshal(bodyBytes, body); err != nil {
			log.Println(err)
			return nil, fmt.Errorf("error parsing submission overview from leetcode: %w", err)
		}

		allSubmissions = append(allSubmissions, body.Data.LCSubmissionList.LCSubmissions...)
		if !body.Data.LCSubmissionList.HasNext || body.Data.LCSubmissionList.LastKey == nil {
			break
		}
		lastKey = body.Data.LCSubmissionList.LastKey
	}

	if len(allSubmissions) == 0 {
		return nil, fmt.Errorf("no accepted submissions found for question: %s", titleSlug)
	}

	return allSubmissions, nil
}

func buildSubmissionListQuery(titleSlug string, lastKey *string, limit int, status int) string {
	lastKeyJSON := "null"
	if lastKey != nil {
		lastKeyJSON = fmt.Sprintf("%q", *lastKey)
	}
	return fmt.Sprintf(submissionListQuery, titleSlug, limit, lastKeyJSON, strconv.Itoa(status))
}

func parseSubmissionTimestamp(timestamp string) (time.Time, error) {
	unixTimestamp, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid submission timestamp %q: %w", timestamp, err)
	}
	return time.Unix(unixTimestamp, 0).UTC(), nil
}

func extractTagNames(tags []lcTopicTag) []string {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag.Name == "" {
			continue
		}
		names = append(names, tag.Name)
	}
	return names
}

// Fetches submission's code using the leetcode's submission id
// Uses LC's GraphQl query that's called submissionDetails
// Returns an empty string and an error if it encounters one while querying
func (lc leetcode) fetchSubmissionCode(id string, retry int) (string, error) {
	bodyBytes, err := lc.queryLeetcode(fmt.Sprintf(submissionDetailsQuery, id))
	if err != nil {
		if retry < maxRetry {
			log.Printf("Network error, retry %d/%d after %v\n", retry+1, maxRetry, backoffTime)
			time.Sleep(backoffTime)
			return lc.fetchSubmissionCode(id, retry+1)
		}
		return "", fmt.Errorf("max retries reached for network error: %w", err)
	}

	body := &RequestBody[lcSubmissionDetailsData]{}
	if err := json.Unmarshal(bodyBytes, body); err != nil {
		return "", fmt.Errorf("JSON parsing error: %w", err)
	}

	// Check if we got a null response
	if body.Data.Details == nil {
		if retry < maxRetry {
			log.Printf("Null response, retry %d/%d after %v\n", retry+1, maxRetry, backoffTime)
			time.Sleep(backoffTime)
			return lc.fetchSubmissionCode(id, retry+1)
		}
		log.Printf("Warning: Max retries reached, consistently getting null response for submission %s", id)
		return "", fmt.Errorf("max retries reached for null response%s", id)
	}

	if len(body.Data.Details.Code) == 0 {
		if retry < maxRetry {
			log.Printf("Empty code, retry %d/%d after %v\n", retry+1, maxRetry, backoffTime)
			time.Sleep(backoffTime)
			return lc.fetchSubmissionCode(id, retry+1)
		}
		log.Printf("Warning: Max retries reached with empty code for submission %s", id)
		return "", fmt.Errorf("max retries reached for empty code")
	}

	return body.Data.Details.Code, nil
}

// queryLeetcode sends the query string to leetcode's GraphQL URL (https://leetcode.com/graphql)
//
// On success it returns the resulting bytes of the response body and a nil error
// Otherwise it will return nil and any error it faces while creating the request or while communicating with LC
func (lc leetcode) queryLeetcode(query string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, lc.graphqlUrl, bytes.NewBuffer([]byte(query)))
	if err != nil {
		return nil, err
	}
	lc.addCookieAndHeaders(req)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	bodyBytes, _ := io.ReadAll(res.Body)
	return bodyBytes, nil
}

// Adds cfg.LcCookie cookie and necessary headers to req
func (lc leetcode) addCookieAndHeaders(req *http.Request) {
	cookie := &http.Cookie{
		Name:     "LEETCODE_SESSION",
		Value:    lc.cfg.LcCookie,
		Path:     "/",
		Domain:   ".leetcode.com",
		HttpOnly: true,
		MaxAge:   1209600,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
	}
	req.AddCookie(cookie)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", lc.cfg.LcCookie))
	req.Header.Add("Connection", "keep-alive")
	req.Header.Add("Content-type", "application/json")
}

type RequestBody[T any] struct {
	Data T `json:"data"`
}

type lcUserProgressQuestionListData struct {
	QuestionsList lcUserProgressQuestionList `json:"userProgressQuestionList"`
}

type lcUserProgressQuestionList struct {
	Questions []lcQuestion `json:"questions"`
}

type lcQuestion struct {
	FrontendId      string    `json:"frontendId"`
	Title           string    `json:"title"`
	TitleSlug       string    `json:"titleSlug"`
	LastSubmittedAt time.Time `json:"lastSubmittedAt"`
	QuestionStatus  string    `json:"questionStatus"`
	LastResult      string    `json:"lastResult"`
}

type lcQuestionDetailsData struct {
	Question lcQuestionDetails `json:"question"`
}

type lcQuestionDetails struct {
	Difficulty string       `json:"difficulty"`
	TopicTags  []lcTopicTag `json:"topicTags"`
}

type lcTopicTag struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type lcSubmissionListData struct {
	LCSubmissionList lcSubmissionList `json:"questionSubmissionList"`
}

type lcSubmissionList struct {
	LastKey       *string                `json:"lastKey"`
	HasNext       bool                   `json:"hasNext"`
	LCSubmissions []lcSumbissionOverview `json:"submissions"`
}

type lcSumbissionOverview struct {
	Id        string `json:"id"`
	Lang      string `json:"lang"`
	Timestamp string `json:"timestamp"`
}

type lcSubmissionDetailsData struct {
	Details *lcSubmissionDetails `json:"submissionDetails"`
}

type lcSubmissionDetails struct {
	Code string `json:"code"`
}
