package repository

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

var (
	// Linux paths
	GlobalInkDropRepoDir = "/ftr/userRepositories"

	// macOS paths
	GlobalInkDropRepoDirMac = "/Users/vuongnguyen/Desktop/WORKSPACE/CODING/GOLANG/web-design-repo"

	// Resolved at startup based on OS
	RepoDir     string
	RepoMetaDir string
)

func init() {
	if runtime.GOOS == "darwin" {
		RepoDir = GlobalInkDropRepoDirMac
	} else {
		RepoDir = GlobalInkDropRepoDir
	}
}

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
	var userRepoDir string = fmt.Sprintf("%s/%s/%s", RepoDir, username, reponame)
	var userRepoMetaDir string = fmt.Sprintf("%s/%s/%s", RepoMetaDir, username, reponame)
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
	var userRepoDir string = fmt.Sprintf("%s/%s", RepoDir, userName)
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
	return filepath.Clean(filepath.Join(RepoDir, userName, repoName))
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

// MoveUploadedFile moves a TUS-uploaded file into the user's repository at the
// given working directory, using the original filename. It validates that the
// destination stays within the repository root and cleans up the TUS .info sidecar.
func MoveUploadedFile(userName, repoName, workingDir, filename, tusFilePath string) error {
	root := repositoryRoot(userName, repoName)

	// Ensure root exists
	if ok, _ := DirExists(root); !ok {
		return fmt.Errorf("repository %s/%s does not exist", userName, repoName)
	}

	destDir, err := resolvePathInRepo(root, workingDir, ".")
	if err != nil {
		return fmt.Errorf("invalid working directory: %w", err)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	destPath := filepath.Join(destDir, filepath.Base(filename))
	if !isWithinRoot(root, destPath) {
		return fmt.Errorf("destination path escapes repository root")
	}

	// Move (rename) the uploaded blob to the final destination
	if err := os.Rename(tusFilePath, destPath); err != nil {
		return fmt.Errorf("failed to move uploaded file: %w", err)
	}

	// Clean up the TUS .info sidecar file
	infoFile := tusFilePath + ".info"
	os.Remove(infoFile)

	return nil
}

func DeleteUserRepository(userName string, repoName string) error {
	var mainDirToRemove string = fmt.Sprintf("%s/%s/%s", RepoDir, userName, repoName)
	var metaDirToRemove string = fmt.Sprintf("%s/%s/%s", RepoMetaDir, userName, repoName)

	err1 := os.RemoveAll(mainDirToRemove)
	err2 := os.RemoveAll(metaDirToRemove)

	if err1 != nil || err2 != nil {
		return fmt.Errorf("%s\n%s", err1, err2)
	}

	return nil
}

func PackSQAR(in, user, out string) (string, error) {
	os.MkdirAll(fmt.Sprintf("/ftr/tmp/%s", user), 0755)
	fullArchivePath := fmt.Sprintf("/ftr/tmp/%s/%s.sqar", user, out)
	err := filepath.WalkDir(in, func(path string, d fs.DirEntry, err2 error) error {
		if err2 != nil {
			return nil
		}
		if !d.IsDir() {
			return fmt.Errorf("File found.")
		}
		return nil
	})
	if err == nil {
		return "", fmt.Errorf("No files in repository.")
	}
	_, err = exec.Command("sqar", "pack", "-PCI", in, "-O", fullArchivePath).Output()
	if err != nil {
		return "", err
	}

	// Test if archive exists
	_, err = os.Stat(fullArchivePath)
	if err != nil {
		return "", err
	}

	return fullArchivePath, nil
}
