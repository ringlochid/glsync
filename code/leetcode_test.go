package code

import (
	_ "embed"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ahmed-e-abdulaziz/glsync/config"
	"github.com/stretchr/testify/assert"
)

//go:embed leetcode-testdata/leetcode-responses/question-submission-list-response.json
var questionSubmissionListResponse []byte

//go:embed leetcode-testdata/leetcode-responses/submission-details-response.json
var submissionDetailsResponse []byte

//go:embed leetcode-testdata/leetcode-responses/user-progress-question-list-response.json
var userProgressQuestionListResponse []byte

//go:embed leetcode-testdata/leetcode-responses/question-details-response.json
var questionDetailsResponse []byte

var (
	submissionListCalled           = false
	submissionDetailsCalled        = false
	userProgressQuestionListCalled = false
)

var (
	lc             leetcode
	currentHandler func(w http.ResponseWriter, reqBody string)
)

func TestMain(m *testing.M) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqBody, _ := io.ReadAll(r.Body)
		currentHandler(w, string(reqBody))
	}))
	testURL := "http://" + server.Listener.Addr().String()
	cfg := config.Config{LcCookie: "COOKIE", RepoUrl: "REPO_URL"}
	lc = NewLeetCode(cfg, testURL)
	m.Run()
}

func TestFetchSubmissions(t *testing.T) {
	// Given
	currentHandler = func(w http.ResponseWriter, reqBody string) {
		if strings.Contains(reqBody, "userProgressQuestionList") {
			userProgressQuestionListCalled = true
			_, err := w.Write(userProgressQuestionListResponse)
			if err != nil {
				t.Fatal("Couldn't write userProgressQuestionListResponse to response correctly")
			}
		}
		if strings.Contains(reqBody, "questionData") {
			_, err := w.Write(questionDetailsResponse)
			if err != nil {
				t.Fatal("Couldn't write questionDetailsResponse to response correctly")
			}
		}
		if strings.Contains(reqBody, "submissionList") {
			submissionListCalled = true
			_, err := w.Write(questionSubmissionListResponse)
			if err != nil {
				t.Fatal("Couldn't write questionSubmissionListResponse to response correctly")
			}
		}
		if strings.Contains(reqBody, "submissionDetails") {
			submissionDetailsCalled = true
			_, err := w.Write(submissionDetailsResponse)
			if err != nil {
				t.Fatal("Couldn't write submissionDetailsResponse to response correctly")
			}
		}
	}

	// When
	res, err := lc.FetchSubmissions()
	submission := res[0]

	// Then
	assert.NoError(t, err)
	assert.Equal(t, submission.Id, "128")
	assert.Equal(t, submission.Lang, "golang")
	assert.Equal(t, submission.Title, "Longest Consecutive Sequence")
	assert.Equal(t, "Hard", submission.Difficulty)
	assert.Equal(t, []string{"Array", "Hash Table"}, submission.Tags)
	assert.True(t, userProgressQuestionListCalled)
	assert.True(t, submissionListCalled)
	assert.True(t, submissionDetailsCalled)
}

func TestFetchSubmissionsShouldFetchAllAcceptedSubmissionsAndUseAcceptedStatus(t *testing.T) {
	// Given
	submissionListCalls := 0
	currentHandler = func(w http.ResponseWriter, reqBody string) {
		if strings.Contains(reqBody, "userProgressQuestionList") {
			_, err := w.Write(userProgressQuestionListResponse)
			if err != nil {
				t.Fatal(err)
			}
			return
		}
		if strings.Contains(reqBody, "questionData") {
			_, err := w.Write(questionDetailsResponse)
			if err != nil {
				t.Fatal(err)
			}
			return
		}
		if strings.Contains(reqBody, "submissionList") {
			submissionListCalls++
			assert.Contains(t, reqBody, `"status": 10`)
			if submissionListCalls == 1 {
				assert.Contains(t, reqBody, `"lastKey": null`)
				nextKey := "page-2"
				response := RequestBody[lcSubmissionListData]{
					Data: lcSubmissionListData{
						LCSubmissionList: lcSubmissionList{
							LastKey: &nextKey,
							HasNext: true,
							LCSubmissions: []lcSumbissionOverview{
								{Id: "1", Lang: "golang", Timestamp: "1735406731"},
								{Id: "2", Lang: "golang", Timestamp: "1735406831"},
							},
						},
					},
				}
				assert.NoError(t, json.NewEncoder(w).Encode(response))
				return
			}
			assert.Contains(t, reqBody, `"lastKey": "page-2"`)
			response := RequestBody[lcSubmissionListData]{
				Data: lcSubmissionListData{
					LCSubmissionList: lcSubmissionList{
						LastKey: nil,
						HasNext: false,
						LCSubmissions: []lcSumbissionOverview{
							{Id: "3", Lang: "golang", Timestamp: "1735406931"},
						},
					},
				},
			}
			assert.NoError(t, json.NewEncoder(w).Encode(response))
			return
		}
		if strings.Contains(reqBody, "submissionDetails") {
			_, err := w.Write(submissionDetailsResponse)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	// When
	res, err := lc.FetchSubmissions()

	// Then
	assert.NoError(t, err)
	assert.Len(t, res, 3)
	assert.Equal(t, 2, submissionListCalls)
	assert.Equal(t, time.Unix(1735406731, 0).UTC(), res[0].LastSubmittedAt)
	assert.Equal(t, time.Unix(1735406931, 0).UTC(), res[2].LastSubmittedAt)
}

func TestFetchSubmissionsShouldReturnErrorWhenFetchQuestionsFails(t *testing.T) {
	// Given
	currentHandler = func(w http.ResponseWriter, reqBody string) {
		if strings.Contains(reqBody, "userProgressQuestionList") {
			panic("panicing so the method fetchSubmissionCode fails")
		}
	}

	// When
	_, err := lc.FetchSubmissions()

	// Then
	assert.Error(t, err)
}

func TestFetchSubmissionsShouldReturnErrorWhenFetchSubmissionOverviewFails(t *testing.T) {
	// Given
	currentHandler = func(w http.ResponseWriter, reqBody string) {
		if strings.Contains(reqBody, "userProgressQuestionList") {
			_, err := w.Write(userProgressQuestionListResponse)
			if err != nil {
				t.Error(err)
			}
		}
		if strings.Contains(reqBody, "submissionList") {
			panic("panicing so the method fetchSubmissionCode fails")
		}
	}

	// When
	_, err := lc.FetchSubmissions()

	// Then
	assert.Error(t, err)
}

func TestFetchSubmissionsShouldReturnErrorWhenFetchSubmissionCodeFails(t *testing.T) {
	// Given
	currentHandler = func(w http.ResponseWriter, reqBody string) {
		if strings.Contains(reqBody, "userProgressQuestionList") {
			_, err := w.Write(userProgressQuestionListResponse)
			if err != nil {
				t.Error(err)
			}
		}
		if strings.Contains(reqBody, "questionData") {
			_, err := w.Write(questionDetailsResponse)
			if err != nil {
				t.Error(err)
			}
		}
		if strings.Contains(reqBody, "submissionList") {
			_, err := w.Write(questionSubmissionListResponse)
			if err != nil {
				t.Error(err)
			}
		}
		if strings.Contains(reqBody, "submissionDetails") {
			panic("panicing so the method fetchSubmissionCode fails")
		}
	}

	// When
	_, err := lc.FetchSubmissions()

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch any submissions successfully")
}

func TestFetchSubmissionsShouldRetryAndFailWithEmptySubmissionDataDetails(t *testing.T) {
	// Given
	attemptCount := 0
	currentHandler = func(w http.ResponseWriter, reqBody string) {
		if strings.Contains(reqBody, "submissionDetails") {
			response := RequestBody[lcSubmissionDetailsData]{
				Data: lcSubmissionDetailsData{
					Details: nil,
				},
			}
			json.NewEncoder(w).Encode(response)
			attemptCount++
		}
	}

	// When
	code, err := lc.fetchSubmissionCode("123", 0)

	// Then
	assert.Error(t, err)
	assert.Empty(t, code)
	assert.Equal(t, maxRetry+1, attemptCount)
	assert.Contains(t, err.Error(), "null response for submission 123")
}

func TestFetchSubmissionsShouldRetryAndSucceedWithValidCode(t *testing.T) {
	// Given
	attemptCount := 0
	currentHandler = func(w http.ResponseWriter, reqBody string) {
		if strings.Contains(reqBody, "submissionDetails") {
			attemptCount++
			response := RequestBody[lcSubmissionDetailsData]{
				Data: lcSubmissionDetailsData{
					Details: &lcSubmissionDetails{
						Code: "",
					},
				},
			}
			if attemptCount > 3 {
				response.Data.Details.Code = "valid code"
			}
			json.NewEncoder(w).Encode(response)
		}
	}

	// When
	code, err := lc.fetchSubmissionCode("123", 0)

	// Then
	assert.NoError(t, err)
	assert.Equal(t, "valid code", code)
	assert.Equal(t, 4, attemptCount)
}

func TestFetchSubmissionsShouldRetryAndFailWithEmptyCode(t *testing.T) {
	// Given
	attemptCount := 0
	currentHandler = func(w http.ResponseWriter, reqBody string) {
		if strings.Contains(reqBody, "submissionDetails") {
			response := RequestBody[lcSubmissionDetailsData]{
				Data: lcSubmissionDetailsData{
					Details: &lcSubmissionDetails{
						Code: "",
					},
				},
			}
			json.NewEncoder(w).Encode(response)
			attemptCount++
		}
	}

	// When
	code, err := lc.fetchSubmissionCode("123", 0)

	// Then
	assert.Error(t, err)
	assert.Empty(t, code)
	assert.Equal(t, maxRetry+1, attemptCount)
}

func TestFetchSubmissionCodesShouldRetryFailedIDsInSecondPass(t *testing.T) {
	attempts := map[string]int{}
	currentHandler = func(w http.ResponseWriter, reqBody string) {
		if !strings.Contains(reqBody, "submissionDetails") {
			return
		}
		submissionID := "1"
		if strings.Contains(reqBody, `"submissionId": 2`) {
			submissionID = "2"
		}
		attempts[submissionID]++
		response := RequestBody[lcSubmissionDetailsData]{
			Data: lcSubmissionDetailsData{
				Details: &lcSubmissionDetails{Code: "code-" + submissionID},
			},
		}
		if submissionID == "2" && attempts[submissionID] <= maxRetry+1 {
			response.Data.Details = nil
		}
		assert.NoError(t, json.NewEncoder(w).Encode(response))
	}

	codes, err := lc.fetchSubmissionCodes([]lcSumbissionOverview{{Id: "1"}, {Id: "2"}})

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"1": "code-1", "2": "code-2"}, codes)
	assert.Equal(t, 1, attempts["1"])
	assert.Equal(t, maxRetry+2, attempts["2"])
}
