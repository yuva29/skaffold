/*
Copyright 2018 The Skaffold Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package build

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/docker"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/v1alpha2"
	"github.com/pkg/errors"
)

func trimTarget(buildTarget string) string {

	//TODO(r2d4): strip off leading //:, bad
	trimmedTarget := strings.TrimPrefix(buildTarget, "//")
	// Useful if root target "//:target"
	trimmedTarget = strings.TrimPrefix(trimmedTarget, ":")

	return trimmedTarget
}

func buildTarPath(buildTarget string) string {
	tarPath := trimTarget(buildTarget)
	tarPath = strings.Replace(tarPath, ":", string(os.PathSeparator), 1)

	return tarPath
}

func buildImageTag(buildTarget string) string {
	imageTag := trimTarget(buildTarget)
	imageTag = strings.TrimPrefix(imageTag, ":")

	//TODO(r2d4): strip off trailing .tar, even worse
	imageTag = strings.TrimSuffix(imageTag, ".tar")

	if strings.Contains(imageTag, ":") {
		return fmt.Sprintf("/%s", imageTag)
	}

	return fmt.Sprintf(":%s", imageTag)
}

func (l *LocalBuilder) buildBazel(ctx context.Context, out io.Writer, a *v1alpha2.Artifact) (string, error) {
	cmd := exec.Command("bazel", "build", a.BazelArtifact.BuildTarget)
	cmd.Dir = a.Workspace
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return "", errors.Wrap(err, "running command")
	}

	tarPath := buildTarPath(a.BazelArtifact.BuildTarget)

	imageTag := buildImageTag(a.BazelArtifact.BuildTarget)

	imageTar, err := os.Open(filepath.Join(a.Workspace, "bazel-bin", tarPath))
	if err != nil {
		return "", errors.Wrap(err, "opening image tarball")
	}
	defer imageTar.Close()

	resp, err := l.api.ImageLoad(ctx, imageTar, false)
	if err != nil {
		return "", errors.Wrap(err, "loading image into docker daemon")
	}
	defer resp.Body.Close()

	err = docker.StreamDockerMessages(out, resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "reading from image load response")
	}

	return fmt.Sprintf("bazel%s", imageTag), nil
}
