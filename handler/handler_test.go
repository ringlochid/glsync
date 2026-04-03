package handler

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ahmed-e-abdulaziz/glsync/code"
	"github.com/ahmed-e-abdulaziz/glsync/mocks/mock_code"
	"github.com/ahmed-e-abdulaziz/glsync/mocks/mock_git"
	"go.uber.org/mock/gomock"
)

func TestExecute(t *testing.T) {
	ctrl, mockCodeClient, mockGitClient := initMocks(t)
	defer ctrl.Finish()

	subs := stubSubmissions()
	gomock.InOrder(
		mockCodeClient.EXPECT().FetchSubmissions().Return(subs, nil).Times(1),
		mockGitClient.EXPECT().
			WriteFile("README.md", gomock.Any()).
			DoAndReturn(func(_ string, content string) error {
				assertContainsAll(t, content,
					"# LeetCode archive",
					"`problems/`",
					"`data/`",
					"`page/`",
					"`scripts/`",
				)
				return nil
			}).
			Times(1),
		mockGitClient.EXPECT().WriteFile("data/.gitkeep", "").Return(nil).Times(1),
		mockGitClient.EXPECT().WriteFile("page/.gitkeep", "").Return(nil).Times(1),
		mockGitClient.EXPECT().WriteFile("scripts/.gitkeep", "").Return(nil).Times(1),
		mockGitClient.EXPECT().
			WriteFile("problems/01-two-sum/README.md", gomock.Any()).
			DoAndReturn(func(_ string, content string) error {
				assertContainsAll(t, content,
					"# 1. Two Sum",
					"- Difficulty: Easy",
					"- Tags: Array, Hash Table",
					"- Accepted submissions: 2",
					"2024-12-15T00-00-00Z__sub-1a.py",
					"2024-12-31T00-00-00Z__sub-1b.go",
				)
				return nil
			}).
			Times(1),
		mockGitClient.EXPECT().
			WriteFile("problems/01-two-sum/2024-12-15T00-00-00Z__sub-1a.py", subs[1].Code).
			Return(nil).
			Times(1),
		mockGitClient.EXPECT().
			WriteFile("problems/01-two-sum/2024-12-31T00-00-00Z__sub-1b.go", subs[0].Code).
			Return(nil).
			Times(1),
		mockGitClient.EXPECT().
			WriteFile("problems/12-add-two-numbers/README.md", gomock.Any()).
			DoAndReturn(func(_ string, content string) error {
				assertContainsAll(t, content,
					"# 12. Add Two Numbers",
					"- Difficulty: Medium",
					"- Tags: Linked List, Math",
					"- Accepted submissions: 1",
					"2024-12-20T00-00-00Z__sub-12a.go",
				)
				return nil
			}).
			Times(1),
		mockGitClient.EXPECT().
			WriteFile("problems/12-add-two-numbers/2024-12-20T00-00-00Z__sub-12a.go", subs[2].Code).
			Return(nil).
			Times(1),
		mockGitClient.EXPECT().
			CommitAll("Sync accepted LeetCode submissions (2 problems, 3 submissions)", gomock.AssignableToTypeOf(time.Time{})).
			Return(nil).
			Times(1),
		mockGitClient.EXPECT().Push().Return(nil).Times(1),
	)

	NewHandler(mockCodeClient, mockGitClient).Execute()
}

func stubSubmissions() []code.Submission {
	return []code.Submission{
		{
			Id:              "1",
			Title:           "Two Sum",
			TitleSlug:       "two-sum",
			SubmissionId:    "sub-1b",
			LastSubmittedAt: parseRFC3339("2024-12-31T00:00:00Z"),
			Lang:            "golang",
			Code:            "package main\n",
			Difficulty:      "Easy",
			Tags:            []string{"Hash Table", "Array"},
		},
		{
			Id:              "1",
			Title:           "Two Sum",
			TitleSlug:       "two-sum",
			SubmissionId:    "sub-1a",
			LastSubmittedAt: parseRFC3339("2024-12-15T00:00:00Z"),
			Lang:            "python3",
			Code:            "print('two sum')\n",
			Difficulty:      "Easy",
			Tags:            []string{"Hash Table", "Array"},
		},
		{
			Id:              "12",
			Title:           "Add Two Numbers",
			TitleSlug:       "add-two-numbers",
			SubmissionId:    "sub-12a",
			LastSubmittedAt: parseRFC3339("2024-12-20T00:00:00Z"),
			Lang:            "golang",
			Code:            "package main\n",
			Difficulty:      "Medium",
			Tags:            []string{"Math", "Linked List"},
		},
	}
}

func TestExecuteShouldPanicWhenFetchSubmissionFails(t *testing.T) {
	ctrl, mockCodeClient, mockGitClient := initMocks(t)
	defer ctrl.Finish()
	mockCodeClient.EXPECT().FetchSubmissions().Return(nil, errors.New("mock error")).Times(1)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic although fetch submissions failed")
		}
	}()
	NewHandler(mockCodeClient, mockGitClient).Execute()
}

func TestExecuteShouldPanicWhenWriteFails(t *testing.T) {
	ctrl, mockCodeClient, mockGitClient := initMocks(t)
	defer ctrl.Finish()

	subs := stubSubmissions()
	gomock.InOrder(
		mockCodeClient.EXPECT().FetchSubmissions().Return(subs, nil).Times(1),
		mockGitClient.EXPECT().WriteFile("README.md", gomock.Any()).Return(errors.New("write failed")).Times(1),
	)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic although git write failed")
		}
	}()
	NewHandler(mockCodeClient, mockGitClient).Execute()
}

func TestExecuteShouldPanicWhenCommitAllFails(t *testing.T) {
	ctrl, mockCodeClient, mockGitClient := initMocks(t)
	defer ctrl.Finish()

	subs := stubSubmissions()
	gomock.InOrder(
		mockCodeClient.EXPECT().FetchSubmissions().Return(subs, nil).Times(1),
		mockGitClient.EXPECT().WriteFile("README.md", gomock.Any()).Return(nil).Times(1),
		mockGitClient.EXPECT().WriteFile("data/.gitkeep", "").Return(nil).Times(1),
		mockGitClient.EXPECT().WriteFile("page/.gitkeep", "").Return(nil).Times(1),
		mockGitClient.EXPECT().WriteFile("scripts/.gitkeep", "").Return(nil).Times(1),
		mockGitClient.EXPECT().WriteFile("problems/01-two-sum/README.md", gomock.Any()).Return(nil).Times(1),
		mockGitClient.EXPECT().WriteFile("problems/01-two-sum/2024-12-15T00-00-00Z__sub-1a.py", subs[1].Code).Return(nil).Times(1),
		mockGitClient.EXPECT().WriteFile("problems/01-two-sum/2024-12-31T00-00-00Z__sub-1b.go", subs[0].Code).Return(nil).Times(1),
		mockGitClient.EXPECT().WriteFile("problems/12-add-two-numbers/README.md", gomock.Any()).Return(nil).Times(1),
		mockGitClient.EXPECT().WriteFile("problems/12-add-two-numbers/2024-12-20T00-00-00Z__sub-12a.go", subs[2].Code).Return(nil).Times(1),
		mockGitClient.EXPECT().CommitAll("Sync accepted LeetCode submissions (2 problems, 3 submissions)", gomock.AssignableToTypeOf(time.Time{})).Return(errors.New("commit failed")).Times(1),
	)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic although git commit failed")
		}
	}()
	NewHandler(mockCodeClient, mockGitClient).Execute()
}

func TestExecuteShouldPanicWhenPushFails(t *testing.T) {
	ctrl, mockCodeClient, mockGitClient := initMocks(t)
	defer ctrl.Finish()

	subs := stubSubmissions()
	gomock.InOrder(
		mockCodeClient.EXPECT().FetchSubmissions().Return(subs, nil).Times(1),
		mockGitClient.EXPECT().WriteFile("README.md", gomock.Any()).Return(nil).Times(1),
		mockGitClient.EXPECT().WriteFile("data/.gitkeep", "").Return(nil).Times(1),
		mockGitClient.EXPECT().WriteFile("page/.gitkeep", "").Return(nil).Times(1),
		mockGitClient.EXPECT().WriteFile("scripts/.gitkeep", "").Return(nil).Times(1),
		mockGitClient.EXPECT().WriteFile("problems/01-two-sum/README.md", gomock.Any()).Return(nil).Times(1),
		mockGitClient.EXPECT().WriteFile("problems/01-two-sum/2024-12-15T00-00-00Z__sub-1a.py", subs[1].Code).Return(nil).Times(1),
		mockGitClient.EXPECT().WriteFile("problems/01-two-sum/2024-12-31T00-00-00Z__sub-1b.go", subs[0].Code).Return(nil).Times(1),
		mockGitClient.EXPECT().WriteFile("problems/12-add-two-numbers/README.md", gomock.Any()).Return(nil).Times(1),
		mockGitClient.EXPECT().WriteFile("problems/12-add-two-numbers/2024-12-20T00-00-00Z__sub-12a.go", subs[2].Code).Return(nil).Times(1),
		mockGitClient.EXPECT().CommitAll("Sync accepted LeetCode submissions (2 problems, 3 submissions)", gomock.AssignableToTypeOf(time.Time{})).Return(nil).Times(1),
		mockGitClient.EXPECT().Push().Return(errors.New("push failed")).Times(1),
	)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic although git push failed")
		}
	}()
	NewHandler(mockCodeClient, mockGitClient).Execute()
}

func initMocks(t *testing.T) (*gomock.Controller, *mock_code.MockCodeClient, *mock_git.MockGitClient) {
	ctrl := gomock.NewController(t)
	mockCodeClient := mock_code.NewMockCodeClient(ctrl)
	mockGitClient := mock_git.NewMockGitClient(ctrl)
	return ctrl, mockCodeClient, mockGitClient
}

func parseRFC3339(timeString string) time.Time {
	timestamp, _ := time.Parse(time.RFC3339, timeString)
	return timestamp
}

func assertContainsAll(t *testing.T, content string, values ...string) {
	t.Helper()
	for _, value := range values {
		if !strings.Contains(content, value) {
			t.Fatalf("expected %q to contain %q", content, value)
		}
	}
}
