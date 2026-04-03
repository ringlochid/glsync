package handler

import (
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ahmed-e-abdulaziz/glsync/code"
	"github.com/ahmed-e-abdulaziz/glsync/git"
)

const problemsRootDir = "problems"

type Handler struct {
	codeClient code.CodeClient
	git        git.GitClient
}

type problemArchive struct {
	Id          string
	Title       string
	TitleSlug   string
	Difficulty  string
	Tags        []string
	Submissions []code.Submission
}

func NewHandler(codeClient code.CodeClient, gitClient git.GitClient) Handler {
	return Handler{codeClient, gitClient}
}

// It does three things:
//
//	1- Fetch submissions using codeClient
//	2- Materialize a clean repo tree with problem folders + all accepted submissions
//	3- Commit once and push the sync result
func (h Handler) Execute() {
	submissions, err := h.codeClient.FetchSubmissions()
	if err != nil {
		panic("Error while fetching code submissions: " + err.Error())
	}

	problems := groupProblems(submissions)
	idWidth := maxProblemIDWidth(problems)
	log.Printf("Fetched %v accepted submissions across %v problems, materializing archive next\n", len(submissions), len(problems))

	if err := h.writeRepoScaffolding(); err != nil {
		panic("Error while writing repo scaffolding: " + err.Error())
	}

	for _, problem := range problems {
		folderName := h.buildFolderName(problem.Id, problem.TitleSlug, idWidth)
		if err := h.git.WriteFile(filepath.Join(folderName, "README.md"), buildProblemReadme(problem)); err != nil {
			panic("Error while writing problem README: " + err.Error())
		}

		for _, submission := range problem.Submissions {
			filePath := filepath.Join(folderName, h.buildSubmissionFileName(submission))
			if err := h.git.WriteFile(filePath, submission.Code); err != nil {
				panic("Error while writing submission file: " + err.Error())
			}
		}
	}

	commitMessage := fmt.Sprintf("Sync accepted LeetCode submissions (%d problems, %d submissions)", len(problems), len(submissions))
	if err := h.git.CommitAll(commitMessage, time.Now().UTC()); err != nil {
		panic("Encountered an error while creating the sync commit: " + err.Error())
	}
	if err := h.git.Push(); err != nil {
		panic("Encountered an error while pushing to git, exiting...")
	}
}

func groupProblems(submissions []code.Submission) []problemArchive {
	grouped := make(map[string]*problemArchive)
	for _, submission := range submissions {
		problem, ok := grouped[submission.Id]
		if !ok {
			tags := append([]string(nil), submission.Tags...)
			sort.Strings(tags)
			problem = &problemArchive{
				Id:         submission.Id,
				Title:      submission.Title,
				TitleSlug:  submission.TitleSlug,
				Difficulty: submission.Difficulty,
				Tags:       tags,
			}
			grouped[submission.Id] = problem
		}
		problem.Submissions = append(problem.Submissions, submission)
	}

	problems := make([]problemArchive, 0, len(grouped))
	for _, problem := range grouped {
		sort.SliceStable(problem.Submissions, func(i, j int) bool {
			if problem.Submissions[i].LastSubmittedAt.Equal(problem.Submissions[j].LastSubmittedAt) {
				return problem.Submissions[i].SubmissionId < problem.Submissions[j].SubmissionId
			}
			return problem.Submissions[i].LastSubmittedAt.Before(problem.Submissions[j].LastSubmittedAt)
		})
		problems = append(problems, *problem)
	}

	sort.SliceStable(problems, func(i, j int) bool {
		left, leftErr := strconv.Atoi(problems[i].Id)
		right, rightErr := strconv.Atoi(problems[j].Id)
		if leftErr == nil && rightErr == nil {
			return left < right
		}
		return problems[i].Id < problems[j].Id
	})

	return problems
}

func maxProblemIDWidth(problems []problemArchive) int {
	width := 1
	for _, problem := range problems {
		if len(problem.Id) > width {
			width = len(problem.Id)
		}
	}
	return width
}

func buildProblemReadme(problem problemArchive) string {
	difficulty := problem.Difficulty
	if difficulty == "" {
		difficulty = "Unknown"
	}
	tags := "None"
	if len(problem.Tags) > 0 {
		tags = strings.Join(problem.Tags, ", ")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s. %s\n\n", problem.Id, problem.Title)
	fmt.Fprintf(&b, "- Difficulty: %s\n", difficulty)
	fmt.Fprintf(&b, "- Tags: %s\n", tags)
	fmt.Fprintf(&b, "- Accepted submissions: %d\n\n", len(problem.Submissions))
	b.WriteString("## Accepted submission files\n\n")
	for _, submission := range problem.Submissions {
		fmt.Fprintf(&b, "- `%s` — %s — %s\n", buildSubmissionFileName(submission), strings.ToUpper(languageLabel(submission.Lang)), submission.LastSubmittedAt.UTC().Format(time.RFC3339))
	}
	return b.String()
}

func (h Handler) writeRepoScaffolding() error {
	type scaffoldFile struct {
		path    string
		content string
	}

	scaffoldingFiles := []scaffoldFile{
		{path: "README.md", content: buildRootReadme()},
		{path: filepath.Join("data", ".gitkeep"), content: ""},
		{path: filepath.Join("page", ".gitkeep"), content: ""},
		{path: filepath.Join("scripts", ".gitkeep"), content: ""},
	}

	for _, file := range scaffoldingFiles {
		if err := h.git.WriteFile(file.path, file.content); err != nil {
			return err
		}
	}

	return nil
}

func buildRootReadme() string {
	return strings.TrimSpace(`
# LeetCode archive

This repository is generated by the local leetcode sync workflow.

## Layout

- ` + "`problems/`" + ` — accepted submissions grouped by problem
- ` + "`data/`" + ` — reserved for future generated data artifacts
- ` + "`page/`" + ` — reserved for future generated site/page output
- ` + "`scripts/`" + ` — reserved for future generated helper assets

Each problem lives under ` + "`problems/<id>-<title-slug>/`" + ` and includes:

- a per-problem ` + "`README.md`" + `
- all accepted submission files for that problem
`) + "\n"
}

func (Handler) buildFolderName(id, titleSlug string, width int) string {
	return filepath.Join(problemsRootDir, fmt.Sprintf("%0*d-%s", width, parseProblemID(id), titleSlug))
}

func parseProblemID(id string) int {
	parsed, err := strconv.Atoi(id)
	if err != nil {
		return 0
	}
	return parsed
}

func buildSubmissionFileName(submission code.Submission) string {
	timestamp := submission.LastSubmittedAt.UTC().Format("2006-01-02T15-04-05Z")
	ext := languageExtension(submission.Lang)
	return fmt.Sprintf("%s__%s.%s", timestamp, submission.SubmissionId, ext)
}

func (h Handler) buildSubmissionFileName(submission code.Submission) string {
	return buildSubmissionFileName(submission)
}

func languageLabel(lang string) string {
	if lang == "" {
		return "text"
	}
	return lang
}

func languageExtension(lang string) string {
	langFileExtension := map[string]string{
		"cpp":        "cpp",
		"java":       "java",
		"python":     "py",
		"python3":    "py",
		"mysql":      "sql",
		"mssql":      "sql",
		"oraclesql":  "sql",
		"c":          "c",
		"csharp":     "cs",
		"javascript": "js",
		"typescript": "ts",
		"bash":       "sh",
		"php":        "php",
		"swift":      "swift",
		"kotlin":     "kt",
		"dart":       "dart",
		"golang":     "go",
		"ruby":       "rb",
		"scala":      "scala",
		"rust":       "rs",
		"racket":     "rkt",
		"erlang":     "erl",
		"elixir":     "ex",
		"postgresql": "sql",
	}
	if ext, ok := langFileExtension[lang]; ok {
		return ext
	}
	return "txt"
}
