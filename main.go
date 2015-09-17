// TODO: don't hold on to everything forever.
// TODO: better auth story, prompt for oauth access and store it somewhere.
// TODO: support writing files if the ref is a branch.
// TODO: better docs, examples, tests, the usual.
package main

import (
	"encoding/base64"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/google/go-github/github"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

var (
	token      = flag.String("token", "", "GitHub auth token")
	mountpoint = flag.String("mountpoint", "", "Mount point, default is current working directory")

	client *github.Client
)

func main() {
	flag.Parse()

	if *token == "" {
		log.Println("must provide -token")
		os.Exit(1)
	}
	client = github.NewClient(oauth2.NewClient(oauth2.NoContext, oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *token},
	)))

	mp := *mountpoint
	if mp == "" {
		mp, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	}
	c, err := fuse.Mount(mp)
	if err != nil {
		log.Printf("mount: %v", err)
		os.Exit(1)
	}
	defer c.Close()

	if err := fs.Serve(c, FS{}); err != nil {
		log.Printf("serve: %v", err)
		os.Exit(1)
	}

	<-c.Ready
	if err := c.MountError; err != nil {
		log.Printf("mount error: %v", err)
		os.Exit(1)
	}
}

// FS represents the filesystem. It serves the root directory.
type FS struct{}

// Root returns the rootDir, which serves the root directory.
func (FS) Root() (fs.Node, error) {
	return &rootDir{}, nil
}

// rootDir serves the root directory.
type rootDir struct{}

// Attr states that a rootDir is a directory.
func (*rootDir) Attr(_ context.Context, attr *fuse.Attr) error {
	*attr = fuse.Attr{Mode: os.ModeDir | 0555}
	return nil
}

// Lookup returns a node with the given name, if it exists.
//
// A node in this context is a user, if one with the name exists.
func (*rootDir) Lookup(_ context.Context, name string) (fs.Node, error) {
	if strings.ContainsRune(name, '.') { // Usernames can't contain '.'
		return nil, fuse.ENOENT
	}
	if _, _, err := client.Users.Get(name); err == nil {
		return &userDir{user: name}, nil
	}
	// If it wasn't a user name, try it as an org name.
	if _, _, err := client.Organizations.Get(name); err == nil {
		return &userDir{user: name}, nil
	}
	return nil, fuse.ENOENT
}

// ReadDirAll returns an empty list, since we can't list all GitHub users.
func (*rootDir) ReadDirAll(context.Context) ([]fuse.Dirent, error) {
	// TODO: return users/orgs we have already fetched instead of nothing
	// at all.
	return nil, nil
}

// userDir serves directories containing a user/org's repos.
type userDir struct {
	user  string
	repos []string
}

// getRepos populates the cache of user's repos if necessary.
func (d *userDir) getRepos() error {
	if d.repos != nil {
		return nil
	}
	repos, resp, err := client.Repositories.List(d.user, nil)
	// Ignore 404s, it may just mean the user is an org.
	if err != nil && resp.StatusCode != http.StatusNotFound {
		log.Println(err)
		return err
	}
	for _, r := range repos {
		d.repos = append(d.repos, *r.Name)
	}

	// Also check if the repos-by-org API returns any repos; there seem to
	// be inconsistent results for orgs, e.g.:
	// https://api.github.com/users/google/repos vs.
	// https://api.github.com/orgs/google/repos
	byOrg, resp, err := client.Repositories.ListByOrg(d.user, nil)
	// Ignore 404s, it may just mean the org is only a user.
	if err != nil && resp.StatusCode != http.StatusNotFound {
		log.Println(err)
		return err
	}
	for _, r := range byOrg {
		d.repos = append(d.repos, *r.Name)
	}
	return nil
}

// Attr states that a userDir represents a directory.
func (d *userDir) Attr(_ context.Context, attr *fuse.Attr) error {
	*attr = fuse.Attr{Mode: os.ModeDir | 0555}
	return nil
}

// Lookup returns a node with the given name, if it exists.
//
// A node in this context is a repo owned by the user/org.
func (d *userDir) Lookup(_ context.Context, name string) (fs.Node, error) {
	if strings.ContainsRune(name, '.') { // Repos can't contain '.'
		return nil, fuse.ENOENT
	}
	if err := d.getRepos(); err != nil {
		return nil, fuse.ENOENT
	}
	for _, r := range d.repos {
		if name == r {
			return &repoDir{userDir: d, repo: r}, nil
		}
	}
	return nil, fuse.ENOENT
}

// ReadDirAll returns a list of user's repos.
func (d *userDir) ReadDirAll(context.Context) ([]fuse.Dirent, error) {
	if err := d.getRepos(); err != nil {
		return nil, fuse.ENOENT
	}
	var ents []fuse.Dirent
	for _, r := range d.repos {
		ents = append(ents, fuse.Dirent{Name: r, Type: fuse.DT_Dir})
	}
	return ents, nil
}

// repoDir serves directories containing a repo's refs.
type repoDir struct {
	*userDir
	repo string
	refs []string
}

// getRefs populates the cache of possible refs if necessary.
//
// TODO: the values of these refs may change if the FS is mounted long-term;
// periodically refresh the list of refs and release things under them.
func (d *repoDir) getRefs() error {
	if d.refs != nil {
		return nil
	}
	tags, _, err := client.Repositories.ListTags(d.user, d.repo, nil)
	if err != nil {
		log.Println(err)
		return err
	}
	for _, t := range tags {
		d.refs = append(d.refs, *t.Name)
	}
	branches, _, err := client.Repositories.ListBranches(d.user, d.repo, nil)
	if err != nil {
		log.Println(err)
		return err
	}
	for _, b := range branches {
		d.refs = append(d.refs, *b.Name)
	}
	return nil
}

// Attr states that a repoDir is a directory.
func (d *repoDir) Attr(_ context.Context, attr *fuse.Attr) error {
	*attr = fuse.Attr{Mode: os.ModeDir | 0555}
	return nil
}

// Lookup returns a node with the given name, if it exists.
//
// A node in this context is a ref, if one with the name exists.
func (d *repoDir) Lookup(_ context.Context, name string) (fs.Node, error) {
	if strings.ContainsRune(name, '.') { // refs can't contain '.'
		return nil, fuse.ENOENT
	}
	if err := d.getRefs(); err != nil {
		return nil, fuse.ENOENT
	}
	for _, r := range d.refs {
		if name == r {
			return &contentDir{repoDir: d, ref: r}, nil
		}
	}
	// TODO: Something is wonky with the GitHub API returning whether or
	// not a commit exists in the repo. For now just always guess it
	// exists and if it doesn't we'll find out later when we try to get a
	// file from it.
	return &contentDir{repoDir: d, ref: name}, nil
}

// ReadDirAll returns a list of repo's refs.
func (d *repoDir) ReadDirAll(context.Context) ([]fuse.Dirent, error) {
	if err := d.getRefs(); err != nil {
		return nil, fuse.ENOENT
	}
	var ents []fuse.Dirent
	for _, r := range d.refs {
		ents = append(ents, fuse.Dirent{Name: r, Type: fuse.DT_Dir})
	}
	return ents, nil
}

// contentDir serves directories and files contained in the repo at a ref.
type contentDir struct {
	*repoDir
	ref, path   string
	files, dirs []string
}

// getContents populates the cache of contents belonging at this path in the
// repo at the ref if necessary.
//
// TODO: contents may change if the FS is mounted long-term (e.g., the parent
// ref "master" changes or is deleted); periodically refresh the contents and
// release things under them.
func (d *contentDir) getContents() error {
	if d.files != nil || d.dirs != nil {
		return nil
	}
	_, contents, _, err := client.Repositories.GetContents(d.user, d.repo, d.path, &github.RepositoryContentGetOptions{d.ref})
	if err != nil {
		log.Println(err)
		return err
	}
	for _, c := range contents {
		if *c.Type == "file" {
			d.files = append(d.files, *c.Name)
		} else if *c.Type == "dir" {
			d.dirs = append(d.dirs, *c.Name)
		}
	}
	return nil
}

// Attr states that a contentDir is a directory.
func (d *contentDir) Attr(_ context.Context, attr *fuse.Attr) error {
	*attr = fuse.Attr{Mode: os.ModeDir | 0555}
	return nil
}

// Lookup returns a node with the given name, if it exists.
//
// A node in this context may be either a further contentDir if the path points
// to a directory in the repo, or it may be a contentFile if it points to a
// file in the repo.
func (d *contentDir) Lookup(_ context.Context, name string) (fs.Node, error) {
	if err := d.getContents(); err != nil {
		return nil, fuse.ENOENT
	}
	for _, f := range d.files {
		if name == f {
			return &contentFile{contentDir: d, filename: filepath.Join(d.path, name)}, nil
		}
	}
	for _, dr := range d.dirs {
		if name == dr {
			return &contentDir{repoDir: d.repoDir, ref: d.ref, path: filepath.Join(d.path, name)}, nil
		}
	}
	return nil, fuse.ENOENT
}

// ReadDirAll returns a list of directories and files in the repo at the ref.
func (d *contentDir) ReadDirAll(context.Context) ([]fuse.Dirent, error) {
	if err := d.getContents(); err != nil {
		return nil, fuse.ENOENT
	}
	var ents []fuse.Dirent
	for _, d := range d.dirs {
		ents = append(ents, fuse.Dirent{Name: d, Type: fuse.DT_Dir})
	}
	for _, f := range d.files {
		ents = append(ents, fuse.Dirent{Name: f, Type: fuse.DT_File})
	}
	return ents, nil
}

// contentFile serves file contents for leaf-node files in a repo at a ref.
type contentFile struct {
	*contentDir // embed user/repo/ref/path
	filename    string
	content     []byte
}

// getFile populates the cache of file contents if necessary.
func (d *contentFile) getFile() error {
	path := filepath.Join(d.path, d.filename)
	contents, _, _, err := client.Repositories.GetContents(d.user, d.repo, path, &github.RepositoryContentGetOptions{d.ref})
	if err != nil {
		log.Println(err)
		return err
	}
	if contents == nil || contents.Content == nil {
		log.Println("nil content")
		return errors.New("nil content")
	}
	if *contents.Encoding == "base64" {
		l := base64.StdEncoding.DecodedLen(len(*contents.Content))
		d.content = make([]byte, l)
		n, err := base64.StdEncoding.Decode(d.content, []byte(*contents.Content))
		if err != nil {
			log.Println(err)
			return err
		}
		d.content = d.content[0:n] // trim any padding
	} else {
		d.content = []byte(*contents.Content)
	}
	return nil
}

// Attr states that contentFile is a file and provides its size.
func (d *contentFile) Attr(_ context.Context, attr *fuse.Attr) error {
	if err := d.getFile(); err != nil {
		// It's a file; we don't know its size.
		*attr = fuse.Attr{Mode: os.FileMode(0) | 0555}
	} else {
		*attr = fuse.Attr{Size: uint64(len(d.content)), Mode: os.FileMode(0) | 0555}
	}
	return nil
}

// ReadAll returns all of the file's contents.
func (d *contentFile) ReadAll(context.Context) ([]byte, error) {
	if err := d.getFile(); err != nil {
		return nil, fuse.ENOENT
	}
	return d.content, nil
}

// Read responds with a possible subset of the file's contents.
func (d *contentFile) Read(_ context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	if err := d.getFile(); err != nil {
		log.Println(err)
		return err
	}
	*resp = fuse.ReadResponse{
		Data: d.content[req.Offset : req.Offset+int64(req.Size)],
	}
	return nil
}
