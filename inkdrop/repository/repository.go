package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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
	// Shared FtR storage root. Override with FTR_ROOT_DIR when needed.
	GlobalInkDropRootDir = "/srv/ftr"

	// macOS fallback uses the same shared layout by default.
	GlobalInkDropRootDirMac = "/srv/ftr"

	// Resolved at startup based on OS.
	RootDir     string
	RepoDir     string
	RepoMetaDir string
	TempDir     string
)

func init() {
	rootDir := strings.TrimSpace(os.Getenv("FTR_ROOT_DIR"))
	defaultRootDir := GlobalInkDropRootDir
	if runtime.GOOS == "darwin" {
		defaultRootDir = GlobalInkDropRootDirMac
	}
	if rootDir == "" {
		rootDir = defaultRootDir
	}

	RootDir = filepath.Clean(rootDir)
	RepoDir = filepath.Join(RootDir, "userRepositories")
	// RepoMetaDir stores per-repository metadata (meta.json files)
	RepoMetaDir = filepath.Join(RepoDir, "_meta")
	TempDir = filepath.Join(RootDir, "tmp")
}

func EnsureStorageLayout() error {
	for _, dir := range []string{RepoDir, RepoMetaDir, TempDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
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

func SearchRepositories(query string) ([]map[string]string, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}

	all := q == "/" || q == "*"
	q = strings.ToLower(q)
	var results []map[string]string

	users, err := os.ReadDir(RepoDir)
	if err != nil {
		return nil, err
	}

	for _, userDir := range users {
		if !userDir.IsDir() {
			continue
		}
		userName := userDir.Name()
		if strings.HasPrefix(userName, "_") {
			// skip internal/system dirs such as _meta
			continue
		}
		repoDir := filepath.Join(RepoDir, userName)
		repos, err := os.ReadDir(repoDir)
		if err != nil {
			continue
		}
		for _, repoDirEntry := range repos {
			if !repoDirEntry.IsDir() {
				continue
			}
			// skip internal dirs
			if strings.HasPrefix(repoDirEntry.Name(), "_") {
				continue
			}
			repoName := repoDirEntry.Name()
			// Load meta description if available
			desc := ""
			if meta, err := LoadRepoMeta(userName, repoName); err == nil && meta != nil {
				desc = meta.Description
			}
			if all || strings.Contains(strings.ToLower(userName), q) || strings.Contains(strings.ToLower(repoName), q) || strings.Contains(strings.ToLower(desc), q) {
				results = append(results, map[string]string{"user": userName, "repo": repoName, "description": desc})
			}
		}
	}

	return results, nil
}

// ListPublicRepositories returns a list of repositories that have their
// metadata Public flag set. Each item contains 'user', 'repo' and 'description'.
func ListPublicRepositories() ([]map[string]string, error) {
	var results []map[string]string

	users, err := os.ReadDir(RepoDir)
	if err != nil {
		return nil, err
	}

	for _, u := range users {
		if !u.IsDir() {
			continue
		}
		userName := u.Name()
		if strings.HasPrefix(userName, "_") {
			continue
		}
		repos, err := os.ReadDir(filepath.Join(RepoDir, userName))
		if err != nil {
			continue
		}
		for _, re := range repos {
			if !re.IsDir() {
				continue
			}
			repoName := re.Name()
			if strings.HasPrefix(repoName, "_") {
				continue
			}
			if meta, err := LoadRepoMeta(userName, repoName); err == nil && meta != nil {
				if meta.Public {
					results = append(results, map[string]string{"user": userName, "repo": repoName, "description": meta.Description})
				}
			}
		}
	}

	return results, nil
}

// ListSharedRepositories returns repositories where the given username is
// listed as an owner in the repo metadata. Returns items with 'user', 'repo', 'description'.
func ListSharedRepositories(username string) ([]map[string]string, error) {
	var results []map[string]string
	users, err := os.ReadDir(RepoDir)
	if err != nil {
		return nil, err
	}

	for _, u := range users {
		if !u.IsDir() {
			continue
		}
		userName := u.Name()
		if strings.HasPrefix(userName, "_") {
			continue
		}
		repos, err := os.ReadDir(filepath.Join(RepoDir, userName))
		if err != nil {
			continue
		}
		for _, re := range repos {
			if !re.IsDir() {
				continue
			}
			repoName := re.Name()
			if strings.HasPrefix(repoName, "_") {
				continue
			}
			if meta, err := LoadRepoMeta(userName, repoName); err == nil && meta != nil {
				for _, o := range meta.Owners {
					if o == username {
						results = append(results, map[string]string{"user": userName, "repo": repoName, "description": meta.Description})
						break
					}
				}
			}
		}
	}

	return results, nil
}

func ListRepositoryFilesRecursive(userName, repoName string) ([]map[string]interface{}, error) {
	root := repositoryRoot(userName, repoName)
	if ok, err := DirExists(root); err != nil || !ok {
		return nil, fmt.Errorf("repository %s/%s not found", userName, repoName)
	}

	var files []map[string]interface{}
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		files = append(files, map[string]interface{}{"path": filepath.ToSlash(rel), "size": info.Size(), "modified": info.ModTime().Unix()})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	return files, nil
}

func CalculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
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

func GetItemPath(userName, repoName, workingDir, itemName string) (string, error) {
	if itemName == "" || itemName == "." || itemName == ".." {
		return "", errors.New("invalid file name")
	}
	root := repositoryRoot(userName, repoName)
	return resolvePathInRepo(root, workingDir, itemName)
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
	archiveDir := filepath.Join(TempDir, user)
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return "", err
	}
	fullArchivePath := filepath.Join(archiveDir, out+".sqar")
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
