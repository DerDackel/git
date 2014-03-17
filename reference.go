// Copyright (c) 2013 Patrick Gundlach, speedata (Berlin, Germany)
// Copyright 2014 The Gogs Authors.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package git

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Reference struct {
	Name       string
	Oid        *Oid
	dest       string
	repository *Repository
}

var (
	refRexp = regexp.MustCompile("ref: (.*)\n")
)

// not sure if this is needed...
func (ref *Reference) resolveInfo() (*Reference, error) {
	destRef := new(Reference)
	destRef.Name = ref.dest

	destpath := filepath.Join(ref.repository.Path, "info", "refs")
	_, err := os.Stat(destpath)
	if err != nil {
		return nil, err
	}
	infoContents, err := ioutil.ReadFile(destpath)
	if err != nil {
		return nil, err
	}

	r := regexp.MustCompile("([[:xdigit:]]+)\t(.*)\n")
	refs := r.FindAllStringSubmatch(string(infoContents), -1)
	for _, v := range refs {
		if v[2] == ref.dest {
			oid, err := NewOidFromString(v[1])
			if err != nil {
				return nil, err
			}
			destRef.Oid = oid
			return destRef, nil
		}
	}

	return nil, errors.New("Could not resolve info/refs")
}

// AllReferences returns all references of repository.
func (repos *Repository) AllReferences() ([]*Reference, error) {
	dirPath := filepath.Join(repos.Path, "refs/heads")
	f, err := os.Open(dirPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fis, err := f.Readdir(0)
	if err != nil {
		return nil, err
	}

	refs := make([]*Reference, len(fis))
	for i, fi := range fis {
		refs[i] = &Reference{
			repository: repos,
			Name:       fi.Name(),
		}
	}
	return refs, nil
}

// CurrentReference returns current reference of repository.
func (repos *Repository) CurrentReference() (*Reference, error) {
	ref := &Reference{repository: repos}
	f, err := ioutil.ReadFile(filepath.Join(repos.Path, "HEAD"))
	if err != nil {
		return nil, err
	}

	allMatches := refRexp.FindAllStringSubmatch(string(f), 1)
	if allMatches == nil {
		return nil, errors.New("Not yet implemented")
	}
	parts := strings.Split(allMatches[0][0], "/")
	ref.Name = strings.TrimSpace(parts[len(parts)-1])
	return ref, nil
}

// A typical Git repository consists of objects (path objects/ in the root directory)
// and of references to HEAD, branches, tags and such.
func (repos *Repository) LookupReference(name string) (*Reference, error) {
	// First we need to find out what's in the text file. It could be something like
	//     ref: refs/heads/master
	// or just a SHA1 such as
	//     1337a1a1b0694887722f8bd0e541bd0f6567a471
	ref := new(Reference)
	ref.repository = repos
	ref.Name = name
	f, err := ioutil.ReadFile(filepath.Join(repos.Path, name))
	if err != nil {
		return nil, err
	}

	allMatches := refRexp.FindAllStringSubmatch(string(f), 1)
	if allMatches == nil {
		// let's assume this is a SHA1
		oid, err := NewOidFromString(string(f[:40]))
		if err != nil {
			return nil, err
		}
		ref.Oid = oid
		return ref, nil
	}
	// yes, it's "ref: something". Now let's lookup "something"
	ref.dest = allMatches[0][1]
	return repos.LookupReference(ref.dest)
}

// For compatibility with git2go. Return Oid from referece (same as getting .Oid directly)
func (r *Reference) Target() *Oid {
	return r.Oid
}

func (r *Reference) LastCommit() (*Commit, error) {
	return r.repository.LookupCommit(r.Oid)
}

func (r *Reference) CommitsBefore(oid *Oid, limit int) ([]*Commit, error) {
	commit, err := r.repository.LookupCommit(r.Oid)
	if err != nil {
		return nil, err
	}
	var commits []*Commit
	commits = append(commits, commit)
	if commit.ParentCount() > 0 {
		for i := 0; i < commit.ParentCount(); i++ {
			subcommits, err := r.CommitsBefore(commit.Parent(i).Id(), 0)
			if err != nil {
				return nil, err
			}
			commits = append(commits, subcommits...)
		}
	}
	return commits, nil
}

func (r *Reference) AllCommits() ([]*Commit, error) {
	return r.CommitsBefore(r.Oid, 0)
}
