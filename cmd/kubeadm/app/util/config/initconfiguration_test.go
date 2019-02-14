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

package config

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/pmezard/go-difflib/difflib"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmapiv1beta1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta1"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
)

const (
	master_v1alpha3YAML          = "testdata/conversion/master/v1alpha3.yaml"
	master_v1alpha3YAMLNonLinux  = "testdata/conversion/master/v1alpha3_non_linux.yaml"
	master_v1beta1YAML           = "testdata/conversion/master/v1beta1.yaml"
	master_v1beta1YAMLNonLinux   = "testdata/conversion/master/v1beta1_non_linux.yaml"
	master_internalYAML          = "testdata/conversion/master/internal.yaml"
	master_internalYAMLNonLinux  = "testdata/conversion/master/internal_non_linux.yaml"
	master_incompleteYAML        = "testdata/defaulting/master/incomplete.yaml"
	master_defaultedYAML         = "testdata/defaulting/master/defaulted.yaml"
	master_defaultedYAMLNonLinux = "testdata/defaulting/master/defaulted_non_linux.yaml"
	master_invalidYAML           = "testdata/validation/invalid_mastercfg.yaml"
)

func diff(expected, actual []byte) string {
	// Write out the diff
	var diffBytes bytes.Buffer
	difflib.WriteUnifiedDiff(&diffBytes, difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(expected)),
		B:        difflib.SplitLines(string(actual)),
		FromFile: "expected",
		ToFile:   "actual",
		Context:  3,
	})
	return diffBytes.String()
}

func TestLoadInitConfigurationFromFile(t *testing.T) {
	// Create temp folder for the test case
	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Couldn't create tmpdir")
	}
	defer os.RemoveAll(tmpdir)

	// cfgFiles is in cluster_test.go
	var tests = []struct {
		name         string
		fileContents []byte
	}{
		{
			name:         "v1beta1.partial1",
			fileContents: cfgFiles["InitConfiguration_v1beta1"],
		},
		{
			name:         "v1beta1.partial2",
			fileContents: cfgFiles["ClusterConfiguration_v1beta1"],
		},
		{
			name: "v1beta1.full",
			fileContents: bytes.Join([][]byte{
				cfgFiles["InitConfiguration_v1beta1"],
				cfgFiles["ClusterConfiguration_v1beta1"],
				cfgFiles["Kube-proxy_componentconfig"],
				cfgFiles["Kubelet_componentconfig"],
			}, []byte(constants.YAMLDocumentSeparator)),
		},
		{
			name:         "v1alpha3.partial1",
			fileContents: cfgFiles["InitConfiguration_v1alpha3"],
		},
		{
			name:         "v1alpha3.partial2",
			fileContents: cfgFiles["ClusterConfiguration_v1alpha3"],
		},
		{
			name: "v1alpha3.full",
			fileContents: bytes.Join([][]byte{
				cfgFiles["InitConfiguration_v1alpha3"],
				cfgFiles["ClusterConfiguration_v1alpha3"],
				cfgFiles["Kube-proxy_componentconfig"],
				cfgFiles["Kubelet_componentconfig"],
			}, []byte(constants.YAMLDocumentSeparator)),
		},
	}

	for _, rt := range tests {
		t.Run(rt.name, func(t2 *testing.T) {
			cfgPath := filepath.Join(tmpdir, rt.name)
			err := ioutil.WriteFile(cfgPath, rt.fileContents, 0644)
			if err != nil {
				t.Errorf("Couldn't create file")
				return
			}

			obj, err := LoadInitConfigurationFromFile(cfgPath)
			if err != nil {
				t.Errorf("Error reading file: %v", err)
				return
			}

			if obj == nil {
				t.Errorf("Unexpected nil return value")
			}
		})
	}
}

func TestInitConfigurationMarshallingFromFile(t *testing.T) {
	master_v1alpha3YAMLAbstracted := master_v1alpha3YAML
	master_v1beta1YAMLAbstracted := master_v1beta1YAML
	master_internalYAMLAbstracted := master_internalYAML
	master_defaultedYAMLAbstracted := master_defaultedYAML
	if runtime.GOOS != "linux" {
		master_v1alpha3YAMLAbstracted = master_v1alpha3YAMLNonLinux
		master_v1beta1YAMLAbstracted = master_v1beta1YAMLNonLinux
		master_internalYAMLAbstracted = master_internalYAMLNonLinux
		master_defaultedYAMLAbstracted = master_defaultedYAMLNonLinux
	}

	var tests = []struct {
		name, in, out string
		groupVersion  schema.GroupVersion
		expectedErr   bool
	}{
		// These tests are reading one file, loading it using LoadInitConfigurationFromFile that all of kubeadm is using for unmarshal of our API types,
		// and then marshals the internal object to the expected groupVersion
		{ // v1alpha3 -> internal
			name:         "v1alpha3ToInternal",
			in:           master_v1alpha3YAMLAbstracted,
			out:          master_internalYAMLAbstracted,
			groupVersion: kubeadm.SchemeGroupVersion,
		},
		{ // v1beta1 -> internal
			name:         "v1beta1ToInternal",
			in:           master_v1beta1YAMLAbstracted,
			out:          master_internalYAMLAbstracted,
			groupVersion: kubeadm.SchemeGroupVersion,
		},
		{ // v1alpha3 -> internal -> v1beta1
			name:         "v1alpha3Tov1beta1",
			in:           master_v1alpha3YAMLAbstracted,
			out:          master_v1beta1YAMLAbstracted,
			groupVersion: kubeadmapiv1beta1.SchemeGroupVersion,
		},
		{ // v1beta1 -> internal -> v1beta1
			name:         "v1beta1Tov1beta1",
			in:           master_v1beta1YAMLAbstracted,
			out:          master_v1beta1YAMLAbstracted,
			groupVersion: kubeadmapiv1beta1.SchemeGroupVersion,
		},
		// These tests are reading one file that has only a subset of the fields populated, loading it using LoadInitConfigurationFromFile,
		// and then marshals the internal object to the expected groupVersion
		{ // v1beta1 -> default -> validate -> internal -> v1beta1
			name:         "incompleteYAMLToDefaultedv1beta1",
			in:           master_incompleteYAML,
			out:          master_defaultedYAMLAbstracted,
			groupVersion: kubeadmapiv1beta1.SchemeGroupVersion,
		},
		{ // v1alpha3 -> validation should fail
			name:        "invalidYAMLShouldFail",
			in:          master_invalidYAML,
			expectedErr: true,
		},
	}

	for _, rt := range tests {
		t.Run(rt.name, func(t2 *testing.T) {

			internalcfg, err := LoadInitConfigurationFromFile(rt.in)
			if err != nil {
				if rt.expectedErr {
					return
				}
				t2.Fatalf("couldn't unmarshal test data: %v", err)
			}

			actual, err := MarshalInitConfigurationToBytes(internalcfg, rt.groupVersion)
			if err != nil {
				t2.Fatalf("couldn't marshal internal object: %v", err)
			}

			expected, err := ioutil.ReadFile(rt.out)
			if err != nil {
				t2.Fatalf("couldn't read test data: %v", err)
			}

			if !bytes.Equal(expected, actual) {
				t2.Errorf("the expected and actual output differs.\n\tin: %s\n\tout: %s\n\tgroupversion: %s\n\tdiff: \n%s\n",
					rt.in, rt.out, rt.groupVersion.String(), diff(expected, actual))
			}
		})
	}
}

func TestConsistentOrderByteSlice(t *testing.T) {
	var (
		aKind = "Akind"
		aFile = []byte(`
kind: Akind
apiVersion: foo.k8s.io/v1
`)
		aaKind = "Aakind"
		aaFile = []byte(`
kind: Aakind
apiVersion: foo.k8s.io/v1
`)
		abKind = "Abkind"
		abFile = []byte(`
kind: Abkind
apiVersion: foo.k8s.io/v1
`)
	)
	var tests = []struct {
		name     string
		in       map[string][]byte
		expected [][]byte
	}{
		{
			name: "a_aa_ab",
			in: map[string][]byte{
				aKind:  aFile,
				aaKind: aaFile,
				abKind: abFile,
			},
			expected: [][]byte{aaFile, abFile, aFile},
		},
		{
			name: "a_ab_aa",
			in: map[string][]byte{
				aKind:  aFile,
				abKind: abFile,
				aaKind: aaFile,
			},
			expected: [][]byte{aaFile, abFile, aFile},
		},
		{
			name: "aa_a_ab",
			in: map[string][]byte{
				aaKind: aaFile,
				aKind:  aFile,
				abKind: abFile,
			},
			expected: [][]byte{aaFile, abFile, aFile},
		},
		{
			name: "aa_ab_a",
			in: map[string][]byte{
				aaKind: aaFile,
				abKind: abFile,
				aKind:  aFile,
			},
			expected: [][]byte{aaFile, abFile, aFile},
		},
		{
			name: "ab_a_aa",
			in: map[string][]byte{
				abKind: abFile,
				aKind:  aFile,
				aaKind: aaFile,
			},
			expected: [][]byte{aaFile, abFile, aFile},
		},
		{
			name: "ab_aa_a",
			in: map[string][]byte{
				abKind: abFile,
				aaKind: aaFile,
				aKind:  aFile,
			},
			expected: [][]byte{aaFile, abFile, aFile},
		},
	}

	for _, rt := range tests {
		t.Run(rt.name, func(t2 *testing.T) {
			actual := consistentOrderByteSlice(rt.in)
			if !reflect.DeepEqual(rt.expected, actual) {
				t2.Errorf("the expected and actual output differs.\n\texpected: %s\n\tout: %s\n", rt.expected, actual)
			}
		})
	}
}
