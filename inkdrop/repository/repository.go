package repository

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	GlobalInkDropRepoDir     = "/ftr/userRepositories"
	GlobalInkDropRepoMetaDir = "/ftr/userRepositoryMetadata"
)

func DirExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return info.IsDir(), nil // Check if the path points to a directory
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil // Path does not exist
	}
	return false, err // Other error (e.g., permission issues)
}

func ProcessRepoName(raw string) (string, error) {
	rawWithoutSpaces := strings.ReplaceAll(raw, " ", "_")
	pass, err := regexp.MatchString("^[a-zA-Z0-9_-]+$", rawWithoutSpaces)
	if err != nil {
		return "", err
	}
	if pass != true {
		return "", errors.New("repository name contains special characters")
	}
	return rawWithoutSpaces, nil
}

func CreateNewUserRepository(username string, reponame string) error {
	var userRepoDir string = fmt.Sprintf("%s/%s/%s", GlobalInkDropRepoDir, username, reponame)
	var userRepoMetaDir string = fmt.Sprintf("%s/%s/%s", GlobalInkDropRepoMetaDir, username, reponame)
	exists1, err1 := DirExists(userRepoDir)
	exists2, err2 := DirExists(userRepoMetaDir)
	if err1 != nil || err2 != nil {
		return fmt.Errorf("%s\n%s", err1, err2)
	}
	if exists1 || exists2 {
		return errors.New("a path for this repository already exists. Consider renaming the repository")
	}
	err1 = os.MkdirAll(userRepoDir, 0755)
	err2 = os.MkdirAll(userRepoMetaDir, 0755)
	if err1 != nil || err2 != nil {
		return fmt.Errorf("%s\n%s", err1, err2)
	}
	return nil
}

func ListUserRepositories(userName string) ([]string, error) {
	var userRepoDir string = fmt.Sprintf("%s/%s", GlobalInkDropRepoDir, userName)
	entries, err := os.ReadDir(userRepoDir)
	var clean []string
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			clean = append(clean, entry.Name())
		}
	}

	return clean, nil
}

func GetDirectoryListing(userName string, repoName string, path string) ([]string, error) {
	var directoryToList string = fmt.Sprintf("%s/%s/%s%s", GlobalInkDropRepoDir, userName, repoName, path)
	entries, err := os.ReadDir(directoryToList)
	if err != nil {
		return nil, err
	}
	var clean []string

	for _, entry := range entries {
		clean = append(clean, entry.Name())
	}

	return clean, nil
}

// func UploadFile(userName string, repoName string, fileName string, destPath string) ([]string, error)

func DeleteUserRepository(userName string, repoName string) error {
	var mainDirToRemove string = fmt.Sprintf("%s/%s/%s", GlobalInkDropRepoDir, userName, repoName)
	var metaDirToRemove string = fmt.Sprintf("%s/%s/%s", GlobalInkDropRepoMetaDir, userName, repoName)

	err1 := os.RemoveAll(mainDirToRemove)
	err2 := os.RemoveAll(metaDirToRemove)

	if err1 != nil || err2 != nil {
		return fmt.Errorf("%s\n%s", err1, err2)
	}

	return nil
}
