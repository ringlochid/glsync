package integration_test

import (
	_ "embed"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ahmed-e-abdulaziz/glsync/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var questionSubmissionListResponse, _ = os.ReadFile("../code/leetcode-testdata/leetcode-responses/question-submission-list-response.json")

var submissionDetailsResponse, _ = os.ReadFile("../code/leetcode-testdata/leetcode-responses/submission-details-response.json")

var userProgressQuestionListResponse, _ = os.ReadFile("../code/leetcode-testdata/leetcode-responses/user-progress-question-list-response.json")

var questionDetailsResponse, _ = os.ReadFile("../code/leetcode-testdata/leetcode-responses/question-details-response.json")

var userProgressQuestionListCalled, submissionListCalled, submissionDetailsCalled bool

func TestLeetCodeGitIntegration(t *testing.T) {
	// Given
	mockLeetCodeUrl := initMockLeetCode(t)
	mockGitRepoUrl := initStubRepo(t)
	os.Args = append(os.Args, "-lc-cookie=eyJhbGciOiJIUzI1NiJ9.eyJuYW1lIjoiWW91IGZvdW5kIGEgc2VjcmV0ISJ9.bg7oA5pFvtjAn1cXW7RRVhl0MUpJqmb90AUiRjh5XHY") // Fake but valid JWT, like LeetCode's cookie
	os.Args = append(os.Args, "-repo-url="+mockGitRepoUrl)
	defer os.RemoveAll("repo")

	// When
	cmd.Execute(mockLeetCodeUrl)

	// Then
	// Assert LeetCode called as expected
	assert.True(t, userProgressQuestionListCalled)
	assert.True(t, submissionListCalled)
	assert.True(t, submissionDetailsCalled)

	// Assert the Git push worked successfully with one sync commit
	_, message := getCommitTimeAndMessage(t, mockGitRepoUrl)
	assert.Equal(t, "Sync accepted LeetCode submissions (1 problems, 1 submissions)", message)
	assertRemoteFileExists(t, mockGitRepoUrl, "README.md")
	assertRemoteFileExists(t, mockGitRepoUrl, "data/.gitkeep")
	assertRemoteFileExists(t, mockGitRepoUrl, "page/.gitkeep")
	assertRemoteFileExists(t, mockGitRepoUrl, "scripts/.gitkeep")
	assertRemoteFileExists(t, mockGitRepoUrl, "problems/128-longest-consecutive-sequence/README.md")
}

func initMockLeetCode(t *testing.T) string {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqBody, _ := io.ReadAll(r.Body)
		if strings.Contains(string(reqBody), "userProgressQuestionList") {
			userProgressQuestionListCalled = true
			_, err := w.Write(userProgressQuestionListResponse)
			if err != nil {
				t.Fatal("Couldn't write userProgressQuestionListResponse to response correctly")
			}
		}
		if strings.Contains(string(reqBody), "questionData") {
			_, err := w.Write(questionDetailsResponse)
			if err != nil {
				t.Fatal("Couldn't write questionDetailsResponse to response correctly")
			}
		}
		if strings.Contains(string(reqBody), "submissionList") {
			submissionListCalled = true
			_, err := w.Write(questionSubmissionListResponse)
			if err != nil {
				t.Fatal("Couldn't write questionSubmissionListResponse to response correctly")
			}
		}
		if strings.Contains(string(reqBody), "submissionDetails") {
			submissionDetailsCalled = true
			_, err := w.Write(submissionDetailsResponse)
			if err != nil {
				t.Fatal("Couldn't write submissionDetailsResponse to response correctly")
			}
		}
	}))
	testUrl := "http://" + server.Listener.Addr().String()
	return testUrl
}

// Creates a simple folder with git init inside to create a repo
func initStubRepo(t *testing.T) string {
	repoPath := "repo/test.git"
	err := os.MkdirAll(repoPath, os.ModePerm)
	require.NoError(t, err)
	err = exec.Command("git", "init", "--bare", repoPath).Run()
	require.NoError(t, err)
	return repoPath
}

func getCommitTimeAndMessage(t *testing.T, repoPath string) (time.Time, string) {
	// git log --pretty=format:'%ad|%s'" --date=iso
	logOutputBytes, err := exec.Command("git", "-C", repoPath, "log", "--pretty=format:'%ad|%s'", "--date=iso").CombinedOutput()
	if err != nil {
		t.Fatal("Failed to do git log command ", logOutputBytes, err)
	}
	logOutput := string(logOutputBytes)
	logOutput = logOutput[1 : len(logOutput)-1]
	commitMessageAndDate := strings.Split(string(logOutput), "|")
	actualTimestamp, _ := time.Parse("2006-01-02 15:04:05 -0700", strings.TrimSpace(commitMessageAndDate[0]))
	actualCommitMessage := strings.TrimSpace(commitMessageAndDate[1])
	return actualTimestamp, actualCommitMessage
}

func assertRemoteFileExists(t *testing.T, repoPath, targetPath string) {
	t.Helper()
	showOutput, err := exec.Command("git", "-C", repoPath, "show", "HEAD:"+filepath.ToSlash(targetPath)).CombinedOutput()
	if err != nil {
		t.Fatalf("expected %s in remote repo, git show failed: %s %v", targetPath, string(showOutput), err)
	}
}
