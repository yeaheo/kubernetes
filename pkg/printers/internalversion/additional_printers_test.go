/*
Copyright 2019 The Kubernetes Authors.

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

package internalversion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	yaml "gopkg.in/yaml.v2"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	yamlserializer "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	genericprinters "k8s.io/cli-runtime/pkg/printers"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/printers"
)

///////////////////////////////////////////////////////////////
//
// These tests do not belong in this package, and they
// should be moved (mostly to cli-runtime). seans3.
//
/////////////////////////////////////////////////////////////

var testData = TestStruct{
	TypeMeta:   metav1.TypeMeta{APIVersion: "foo/bar", Kind: "TestStruct"},
	Key:        "testValue",
	Map:        map[string]int{"TestSubkey": 1},
	StringList: []string{"a", "b", "c"},
	IntList:    []int{1, 2, 3},
}

type TestStruct struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Key               string         `json:"Key"`
	Map               map[string]int `json:"Map"`
	StringList        []string       `json:"StringList"`
	IntList           []int          `json:"IntList"`
}

func (in *TestStruct) DeepCopyObject() runtime.Object {
	panic("never called")
}

type TestPrintType struct {
	Data string
}

func (obj *TestPrintType) GetObjectKind() schema.ObjectKind { return schema.EmptyObjectKind }
func (obj *TestPrintType) DeepCopyObject() runtime.Object {
	if obj == nil {
		return nil
	}
	clone := *obj
	return &clone
}

// TODO(seans3): Move this test to cli-runtime/pkg/printers.
func testPrinter(t *testing.T, printer printers.ResourcePrinter, unmarshalFunc func(data []byte, v interface{}) error) {
	buf := bytes.NewBuffer([]byte{})

	err := printer.PrintObj(&testData, buf)
	if err != nil {
		t.Fatal(err)
	}
	var poutput TestStruct
	// Verify that given function runs without error.
	err = unmarshalFunc(buf.Bytes(), &poutput)
	if err != nil {
		t.Fatal(err)
	}
	// Use real decode function to undo the versioning process.
	poutput = TestStruct{}
	s := yamlserializer.NewDecodingSerializer(testapi.Default.Codec())
	if err := runtime.DecodeInto(s, buf.Bytes(), &poutput); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(testData, poutput) {
		t.Errorf("Test data and unmarshaled data are not equal: %v", diff.ObjectDiff(poutput, testData))
	}

	obj := &v1.Pod{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
	}
	// our decoder defaults, so we should default our expected object as well
	legacyscheme.Scheme.Default(obj)
	buf.Reset()
	printer.PrintObj(obj, buf)
	var objOut v1.Pod
	// Verify that given function runs without error.
	err = unmarshalFunc(buf.Bytes(), &objOut)
	if err != nil {
		t.Fatalf("unexpected error: %#v", err)
	}
	// Use real decode function to undo the versioning process.
	objOut = v1.Pod{}
	if err := runtime.DecodeInto(s, buf.Bytes(), &objOut); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(obj, &objOut) {
		t.Errorf("Unexpected inequality:\n%v", diff.ObjectDiff(obj, &objOut))
	}
}

func yamlUnmarshal(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}

// TODO(seans3): Move this test to cli-runtime/pkg/printers.
func TestYAMLPrinter(t *testing.T) {
	testPrinter(t, genericprinters.NewTypeSetter(legacyscheme.Scheme).ToPrinter(&genericprinters.YAMLPrinter{}), yamlUnmarshal)
}

// TODO(seans3): Move this test to cli-runtime/pkg/printers.
func TestJSONPrinter(t *testing.T) {
	testPrinter(t, genericprinters.NewTypeSetter(legacyscheme.Scheme).ToPrinter(&genericprinters.JSONPrinter{}), json.Unmarshal)
}

// TODO(seans3): Move this test to cli-runtime/pkg/printers.
func TestFormatResourceName(t *testing.T) {
	tests := []struct {
		kind schema.GroupKind
		name string
		want string
	}{
		{schema.GroupKind{}, "", ""},
		{schema.GroupKind{}, "name", "name"},
		{schema.GroupKind{Kind: "Kind"}, "", "kind/"}, // should not happen in practice
		{schema.GroupKind{Kind: "Kind"}, "name", "kind/name"},
		{schema.GroupKind{Group: "group", Kind: "Kind"}, "name", "kind.group/name"},
	}
	for _, tt := range tests {
		if got := formatResourceName(tt.kind, tt.name, true); got != tt.want {
			t.Errorf("formatResourceName(%q, %q) = %q, want %q", tt.kind, tt.name, got, tt.want)
		}
	}
}

func PrintCustomType(obj *TestPrintType, options printers.GenerateOptions) ([]metav1beta1.TableRow, error) {
	return []metav1beta1.TableRow{{Cells: []interface{}{obj.Data}}}, nil
}

func ErrorPrintHandler(obj *TestPrintType, options printers.GenerateOptions) ([]metav1beta1.TableRow, error) {
	return nil, fmt.Errorf("ErrorPrintHandler error")
}

func TestCustomTypePrinting(t *testing.T) {
	columns := []metav1beta1.TableColumnDefinition{{Name: "Data"}}
	generator := printers.NewTableGenerator()
	generator.TableHandler(columns, PrintCustomType)

	obj := TestPrintType{"test object"}
	table, err := generator.GenerateTable(&obj, printers.GenerateOptions{})
	if err != nil {
		t.Fatalf("An error occurred generating the table for custom type: %#v", err)
	}

	expectedTable := &metav1.Table{
		ColumnDefinitions: []metav1.TableColumnDefinition{{Name: "Data"}},
		Rows:              []metav1.TableRow{{Cells: []interface{}{"test object"}}},
	}
	if !reflect.DeepEqual(expectedTable, table) {
		t.Errorf("Error generating table from custom type. Expected (%#v), got (%#v)", expectedTable, table)
	}
}

func TestPrintHandlerError(t *testing.T) {
	columns := []metav1beta1.TableColumnDefinition{{Name: "Data"}}
	generator := printers.NewTableGenerator()
	generator.TableHandler(columns, ErrorPrintHandler)
	obj := TestPrintType{"test object"}
	_, err := generator.GenerateTable(&obj, printers.GenerateOptions{})
	if err == nil || err.Error() != "ErrorPrintHandler error" {
		t.Errorf("Did not get the expected error: %#v", err)
	}
}

func TestNamePrinter(t *testing.T) {
	tests := map[string]struct {
		obj    runtime.Object
		expect string
	}{
		"singleObject": {
			&v1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind: "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			},
			"pod/foo\n"},
		"List": {
			&unstructured.UnstructuredList{
				Object: map[string]interface{}{
					"kind":       "List",
					"apiVersion": "v1",
				},
				Items: []unstructured.Unstructured{
					{
						Object: map[string]interface{}{
							"kind":       "Pod",
							"apiVersion": "v1",
							"metadata": map[string]interface{}{
								"name": "bar",
							},
						},
					},
				},
			},
			"pod/bar\n"},
	}

	printFlags := genericclioptions.NewPrintFlags("").WithTypeSetter(legacyscheme.Scheme).WithDefaultOutput("name")
	printer, err := printFlags.ToPrinter()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	for name, item := range tests {
		buff := &bytes.Buffer{}
		err := printer.PrintObj(item.obj, buff)
		if err != nil {
			t.Errorf("%v: unexpected err: %v", name, err)
			continue
		}
		got := buff.String()
		if item.expect != got {
			t.Errorf("%v: expected %v, got %v", name, item.expect, got)
		}
	}
}

// TODO(seans3): Move this test to cli-runtime/pkg/printers.
func TestPrinters(t *testing.T) {
	om := func(name string) metav1.ObjectMeta { return metav1.ObjectMeta{Name: name} }

	var (
		err             error
		jsonpathPrinter printers.ResourcePrinter
	)

	jsonpathPrinter, err = genericprinters.NewJSONPathPrinter("{.metadata.name}")
	if err != nil {
		t.Fatal(err)
	}

	genericPrinters := map[string]printers.ResourcePrinter{
		// TODO(juanvallejo): move "generic printer" tests to pkg/kubectl/genericclioptions/printers
		"json":     genericprinters.NewTypeSetter(legacyscheme.Scheme).ToPrinter(&genericprinters.JSONPrinter{}),
		"yaml":     genericprinters.NewTypeSetter(legacyscheme.Scheme).ToPrinter(&genericprinters.YAMLPrinter{}),
		"jsonpath": jsonpathPrinter,
	}
	objects := map[string]runtime.Object{
		"pod":             &v1.Pod{ObjectMeta: om("pod")},
		"emptyPodList":    &v1.PodList{},
		"nonEmptyPodList": &v1.PodList{Items: []v1.Pod{{}}},
		"endpoints": &v1.Endpoints{
			Subsets: []v1.EndpointSubset{{
				Addresses: []v1.EndpointAddress{{IP: "127.0.0.1"}, {IP: "localhost"}},
				Ports:     []v1.EndpointPort{{Port: 8080}},
			}}},
	}
	// map of printer name to set of objects it should fail on.
	expectedErrors := map[string]sets.String{
		"jsonpath": sets.NewString("emptyPodList", "nonEmptyPodList", "endpoints"),
	}

	for pName, p := range genericPrinters {
		for oName, obj := range objects {
			b := &bytes.Buffer{}
			if err := p.PrintObj(obj, b); err != nil {
				if set, found := expectedErrors[pName]; found && set.Has(oName) {
					// expected error
					continue
				}
				t.Errorf("printer '%v', object '%v'; error: '%v'", pName, oName, err)
			}
		}
	}
}
