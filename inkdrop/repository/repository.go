package repository

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	GlobalInkDropRepoDir     = "/ftr/userRepositories"
	GlobalInkDropRepoMetaDir = "/ftr/userRepositoryMetadata"
)

func DirExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return info.IsDir(), nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
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

func CreateNewDirectory(userName, repoName, workingDir, folderName string) error {
	root := repositoryRoot(userName, repoName)
	target, err := resolvePathInRepo(root, workingDir, folderName)
	if err != nil {
		return err
	}
	err = os.MkdirAll(target, 0755)
	if err != nil {
		return err
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
	root := repositoryRoot(userName, repoName)
	directoryToList, err := resolvePathInRepo(root, path, ".")
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(directoryToList)
	if err != nil {
		return nil, err
	}
	var clean []string

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		clean = append(clean, name)
	}

	sort.Strings(clean)

	return clean, nil
}

func RenameItem(userName, repoName, workingDir, oldName, newName string) error {
	root := repositoryRoot(userName, repoName)
	oldPath, err := resolvePathInRepo(root, workingDir, oldName)
	if err != nil {
		return err
	}
	newPath, err := resolvePathInRepo(root, workingDir, newName)
	if err != nil {
		return err
	}

	if oldPath == root || newPath == root {
		return errors.New("invalid path")
	}

	if _, err := os.Stat(oldPath); err != nil {
		return err
	}
	if _, err := os.Stat(newPath); err == nil {
		return nil
	}

	err = os.MkdirAll(filepath.Dir(newPath), 0755)
	if err != nil {
		return err
	}

	err = os.Rename(oldPath, newPath)
	if err != nil {
		return err
	}

	return nil
}

func DeleteItem(userName, repoName, workingDir, name string) error {
	root := repositoryRoot(userName, repoName)
	target, err := resolvePathInRepo(root, workingDir, name)
	if err != nil {
		return err
	}
	if target == root {
		return errors.New("cannot delete repository root")
	}

	return os.RemoveAll(target)
}

func repositoryRoot(userName, repoName string) string {
	return filepath.Clean(filepath.Join(GlobalInkDropRepoDir, userName, repoName))
}

func normalizeWorkingDir(path string) string {
	if path == "" {
		return "/"
	}
	p := filepath.ToSlash(filepath.Clean("/" + strings.TrimSpace(path)))
	if p == "." || p == "" {
		return "/"
	}
	return p
}

func resolvePathInRepo(root, workingDir, name string) (string, error) {
	rootClean := filepath.Clean(root)

	wd := normalizeWorkingDir(workingDir)
	workingAbs := filepath.Clean(filepath.Join(rootClean, strings.TrimPrefix(wd, "/")))
	if !isWithinRoot(rootClean, workingAbs) {
		return "", errors.New("working directory is outside repository root")
	}

	if name == "" || name == "." {
		return workingAbs, nil
	}

	name = filepath.ToSlash(strings.TrimSpace(name))
	if name == "." || name == ".." || strings.HasPrefix(name, "/") {
		return "", errors.New("invalid target path")
	}

	target := filepath.Clean(filepath.Join(workingAbs, name))
	if !isWithinRoot(rootClean, target) {
		return "", errors.New("target path is outside repository root")
	}

	return target, nil
}

func isWithinRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

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
