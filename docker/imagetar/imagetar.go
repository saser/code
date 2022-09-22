// Package imagetar contains functions for reading tarballs containing Docker
// images and repositories.
package imagetar

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var (
	// ErrRepositoriesNotFound is returned from Repositories when the file
	// "repositories" is not found at the root of the archive.
	ErrRepositoriesNotFound = errors.New("imagetar: repositories file not found")

	// ErrRepositoriesInvalid is returned from Repositories when the file
	// "repositories" is found but does not have the expected JSON structure.
	ErrRepositoriesInvalid = errors.New("imagetar: repositories file is invalid")
)

// Repositories reads out the "repositories" file from the root of the archive
// and parses its contents, which is expected to be JSON, into a map. The map is
// structed as follows to match the definition of the "repositories" file as
// described at https://docs.docker.com/engine/api/v1.41/#operation/ImageGet.
//
//	repository -> tag -> layer ID
//
// If no "repositories" file is found, Repositories returns
// ErrRepositoriesNotFound. If the file is found but its contents cannot be
// parsed as JSON, Repositories returns ErrRepositoriesInvalid.
func Repositories(r io.Reader) (map[string]map[string]string, error) {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, ErrRepositoriesNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("imagetar: read repositories: %w", err)
		}
		if hdr.Name != "repositories" {
			continue
		}
		contents, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("imagetar: read repositories: %w", err)
		}
		repositories := make(map[string]map[string]string)
		if err := json.Unmarshal(contents, &repositories); err != nil {
			return nil, ErrRepositoriesInvalid
		}
		return repositories, nil
	}
}

// Images parses the "repositories" file at the root of the archive and returns
// a list of image names contained in that archive. The strings will have the
// format "path/to/repo:tag".
func Images(r io.Reader) ([]string, error) {
	repos, err := Repositories(r)
	if err != nil {
		return nil, err
	}
	var images []string
	for repo, tags := range repos {
		for tag := range tags {
			images = append(images, repo+":"+tag)
		}
	}
	return images, nil
}
