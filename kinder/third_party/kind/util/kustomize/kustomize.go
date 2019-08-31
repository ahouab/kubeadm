/*
Copyright 2018 The Kubernetes Authors.

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

// Package kustomize contains helpers for working with embedded kustomize commands
package kustomize

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"runtime"

	"sigs.k8s.io/kustomize/v3/k8sdeps/kunstruct"
	"sigs.k8s.io/kustomize/v3/k8sdeps/transformer"
	"sigs.k8s.io/kustomize/v3/k8sdeps/validator"
	"sigs.k8s.io/kustomize/v3/pkg/commands/build"
	"sigs.k8s.io/kustomize/v3/pkg/fs"
	"sigs.k8s.io/kustomize/v3/pkg/resmap"
	"sigs.k8s.io/kustomize/v3/pkg/resource"

	kinderconfig "k8s.io/kubeadm/kinder/pkg/config"
)

// Build takes a set of resource blobs (yaml), patches (strategic merge patch)
// https://github.com/kubernetes/community/blob/master/contributors/devel/strategic-merge-patch.md
// and returns the `kustomize build` result as a yaml blob
// It does this in-memory using the build cobra command
//
// NOTE: this fork of build uses the kinder-aliased, public PatchJSON6902 struct instead of forking the kind-internal one
func Build(resources, patches []string, patchesJSON6902 []kinderconfig.PatchJSON6902) (string, error) {
	// write the resources and patches to an in memory fs with a generated
	// kustomization.yaml
	memFS := fs.MakeFakeFS()
	var kustomization bytes.Buffer
	fakeDir := "/"
	// for Windows we need this to be a drive because kustomize uses filepath.Abs()
	// which will add a drive letter if there is none. which drive letter is
	// unimportant as the path is on the fake filesystem anyhow
	if runtime.GOOS == "windows" {
		fakeDir = `C:\`
	}

	// NOTE: we always write this header as you cannot build without any resources
	kustomization.WriteString("resources:\n")
	for i, resource := range resources {
		// this cannot error per docs
		name := fmt.Sprintf("resource-%d.yaml", i)
		_ = memFS.WriteFile(filepath.Join(fakeDir, name), []byte(resource))
		fmt.Fprintf(&kustomization, " - %s\n", name)
	}

	if len(patches) > 0 {
		kustomization.WriteString("patches:\n")
	}
	for i, patch := range patches {
		// this cannot error per docs
		name := fmt.Sprintf("patch-%d.yaml", i)
		_ = memFS.WriteFile(filepath.Join(fakeDir, name), []byte(patch))
		fmt.Fprintf(&kustomization, " - %s\n", name)
	}

	if len(patchesJSON6902) > 0 {
		kustomization.WriteString("patchesJson6902:\n")
	}
	for i, patch := range patchesJSON6902 {
		// this cannot error per docs
		name := fmt.Sprintf("patch-json6902-%d.yaml", i)
		_ = memFS.WriteFile(filepath.Join(fakeDir, name), []byte(patch.Patch))
		fmt.Fprintf(&kustomization, " - path: %s\n", name)
		fmt.Fprintf(&kustomization, "   target:\n")
		fmt.Fprintf(&kustomization, "     group: %s\n", patch.Group)
		fmt.Fprintf(&kustomization, "     version: %s\n", patch.Version)
		fmt.Fprintf(&kustomization, "     kind: %s\n", patch.Kind)
		if patch.Name != "" {
			fmt.Fprintf(&kustomization, "     name: %s\n", patch.Name)
		}
		if patch.Namespace != "" {
			fmt.Fprintf(&kustomization, "     namespace: %s\n", patch.Namespace)
		}
	}

	memFS.WriteFile(filepath.Join(fakeDir, "kustomization.yaml"), kustomization.Bytes())

	// now we can build the kustomization
	var out bytes.Buffer
	uf := kunstruct.NewKunstructuredFactoryImpl()
	pf := transformer.NewFactoryImpl()
	rf := resmap.NewFactory(resource.NewFactory(uf), pf)
	v := validator.NewKustValidator()
	cmd := build.NewCmdBuild(&out, memFS, v, rf, pf)
	cmd.SetArgs([]string{"--", fakeDir})
	// we want to silence usage, error output, and any future output from cobra
	// we will get error output as a golang error from execute
	cmd.SetOutput(ioutil.Discard)
	_, err := cmd.ExecuteC()
	if err != nil {
		return "", err
	}
	return out.String(), nil
}
