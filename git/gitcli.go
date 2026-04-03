package git

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ahmed-e-abdulaziz/glsync/config"
)

const commitDateEnvVar = "GIT_COMMITTER_DATE"

type gitcli struct {
	cfg            config.Config
	repoFolderName string
}

func NewGitCli(cfg config.Config) gitcli {
	gh := gitcli{cfg: cfg}
	url := strings.Split(gh.cfg.RepoUrl, "/")
	gh.repoFolderName = strings.Split(url[len(url)-1], ".")[0]
	if _, err := os.Stat(gh.repoFolderName); err == nil {
		log.Printf(`Removing folder: [%v] as it is the same as the repo folder's name to be able to clone the repo`,
			gh.repoFolderName)
		os.RemoveAll(gh.repoFolderName)
	}
	log.Printf("Cloning %s next\n", gh.repoFolderName)
	out, err := exec.Command("git", "clone", cfg.RepoUrl).CombinedOutput()
	if err != nil {
		log.Println("Output ", string(out))
		log.Println("Error", err.Error())
		log.Panicf(
			`Encountered an error while cloning the repo.
			Please create your repo on Git before using glsync, the repo: "%s" doesn't exist`,
			cfg.RepoUrl)
	}
	err = os.Chdir(gh.repoFolderName)
	if err != nil {
		log.Panicf("Couldn't chdir into repo folder %s. Please check permissions and try again", gh.repoFolderName)
	}
	if err := gh.clearWorktree(); err != nil {
		log.Panicf("Couldn't clear repo folder %s before sync: %v", gh.repoFolderName, err)
	}
	log.Printf("Cloned %s successfully\n", gh.repoFolderName)
	return gh
}

func (g gitcli) WriteFile(path, content string) error {
	filePath := filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return err
	}
	if err := os.WriteFile(filePath, []byte(content), os.ModePerm); err != nil {
		return err
	}
	return nil
}

func (g gitcli) CommitAll(commitMessage string, timestamp time.Time) error {
	out, err := exec.Command("git", "add", "-A").CombinedOutput()
	if err != nil {
		return fmt.Errorf(`encountered an error while executing the command 'git add -A' in folder %s.
			The error: %s with command output: %s`, g.repoFolderName, err, string(out))
	}
	os.Setenv(commitDateEnvVar, g.toGitDate(timestamp))
	defer os.Unsetenv(commitDateEnvVar)
	out, err = exec.Command("git", "commit", "--allow-empty", fmt.Sprintf("--date=%v", g.toGitDate(timestamp)), "-m", commitMessage).CombinedOutput()
	if err != nil {
		return fmt.Errorf(`encountered an error while executing the command 'git commit --allow-empty --date=%s -m %s' in folder %s.
			The error: %s 
			with command output: %s`,
			g.toGitDate(timestamp), commitMessage, g.repoFolderName, err, string(out))
	}
	return nil
}

func (g gitcli) Push() error {
	err := exec.Command("git", "push").Run()
	if err != nil {
		return errors.New("encountered an error while doing the command 'git push' in the repo folder: " + g.repoFolderName)
	}
	err = os.Chdir("..")
	if err != nil {
		return errors.New("couldn't go back to the enclosing folder 'ch ..', could be a permissions issue")
	}
	err = os.RemoveAll(g.repoFolderName)
	if err != nil {
		return fmt.Errorf("couldn't delete the repo folder after pushing 'rm -rf %s', could be a permissions issue",
			g.repoFolderName)
	}
	return nil
}

func (g gitcli) clearWorktree() error {
	entries, err := os.ReadDir(".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == ".git" {
			continue
		}
		if err := os.RemoveAll(entry.Name()); err != nil {
			return err
		}
	}
	return nil
}

func (g gitcli) toGitDate(timestamp time.Time) string {
	_, offset := timestamp.Zone()
	return fmt.Sprintf("%v %+05d", timestamp.Unix(), offset)
}
