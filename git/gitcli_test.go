package git

import (
	"errors"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ahmed-e-abdulaziz/glsync/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var g gitcli

func TestMain(m *testing.M) {
	testDir := createTestFolder()
	output, err := exec.Command("git", "init").CombinedOutput()
	if err != nil {
		log.Fatal(string(output)+" ", err)
	}
	g = gitcli{config.Config{LcCookie: "COOKIE", RepoUrl: "REPO_URL"}, testDir}
	m.Run()
	deleteTestFolder(testDir)
}

func deleteTestFolder(testDir string) {
	// Go out of the current test folder and delete it
	err := os.Chdir("..")
	if err != nil {
		log.Fatal("Couldn't `cd ..` to delete test folder")
	}
	err = os.RemoveAll(testDir)
	if err != nil {
		log.Fatal("Couldn't delete test folder: " + testDir)
	}
}

func createTestFolder() string {
	// Go to home directory
	home, _ := os.UserHomeDir()

	errArr := []error{os.Chdir(home)}

	// Create and go into the test folder
	testDir := "test-gitcli-folder"
	errArr = append(errArr, os.Mkdir(testDir, os.ModePerm))
	errArr = append(errArr, os.Chdir(home+"/"+testDir))

	err := errors.Join(errArr...)
	if err != nil {
		log.Fatal(err)
	}
	return testDir
}

func TestWriteFile(t *testing.T) {
	// Given
	path, content := "01-two-sum/README.md", "# Two Sum\n"
	defer os.RemoveAll("01-two-sum")

	// When
	err := g.WriteFile(path, content)

	// Then
	assert.NoError(t, err)
	assert.FileExists(t, path)
	actual, _ := os.ReadFile(path)
	assert.Equal(t, content, string(actual))
}

func TestCommitAll(t *testing.T) {
	// Given
	timestamp := time.Now().UTC()
	require.NoError(t, g.WriteFile("01-two-sum/README.md", "# Two Sum\n"))
	defer os.RemoveAll("01-two-sum")

	// When
	err := g.CommitAll("sync commit", timestamp)

	// Then
	assert.NoError(t, err)
	actualTimestamp, actualCommitMessage := getCommitTimeAndMessage(t)
	assert.Equal(t, "sync commit", actualCommitMessage)
	assert.Equal(t, timestamp.UTC().Round(time.Minute).Unix(), actualTimestamp.UTC().Round(time.Minute).Unix())
}

func TestCommitAllShouldFailWhenGitAddFails(t *testing.T) {
	// Given
	originalDir, _ := os.Getwd()
	if err := os.Chdir(os.TempDir()); err != nil {
		t.Error(err)
	}
	require.NoError(t, os.MkdirAll("01-two-sum", os.ModePerm))
	defer os.RemoveAll("01-two-sum")
	require.NoError(t, os.WriteFile("01-two-sum/README.md", []byte("# Two Sum\n"), os.ModePerm))

	// When
	err := g.CommitAll("sync commit", time.Now().UTC())

	// Then
	require.Error(t, err)
	if err = os.Chdir(originalDir); err != nil {
		t.Error(err)
	}
}

func TestCommitAllShouldFailWhenGitCommitFails(t *testing.T) {
	// Given
	require.NoError(t, g.WriteFile("01-two-sum/README.md", "# Two Sum\n"))
	defer os.RemoveAll("01-two-sum")

	// When
	err := g.CommitAll("", time.Now().UTC())

	// Then
	assert.Error(t, err)
}

func getCommitTimeAndMessage(t *testing.T) (time.Time, string) {
	logOutputBytes, err := exec.Command("git", "log", "--pretty=format:'%ad|%s'", "--date=iso").CombinedOutput()
	if err != nil {
		t.Fatal("Failed to do git log command ", string(logOutputBytes), err)
	}
	logOutput := string(logOutputBytes)
	logOutput = logOutput[1 : len(logOutput)-1]
	commitMessageAndDate := strings.Split(string(logOutput), "|")
	actualTimestamp, _ := time.Parse("2006-01-02 15:04:05 -0700", strings.TrimSpace(commitMessageAndDate[0]))
	actualCommitMessage := strings.TrimSpace(commitMessageAndDate[1])
	return actualTimestamp, actualCommitMessage
}
