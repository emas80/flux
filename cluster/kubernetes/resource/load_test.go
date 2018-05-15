package resource

import (
	"bytes"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/weaveworks/flux/cluster/kubernetes/testfiles"
	"github.com/weaveworks/flux/resource"
)

// for convenience
func base(source, kind, namespace, name string) baseObject {
	b := baseObject{source: source, Kind: kind}
	b.Meta.Namespace = namespace
	b.Meta.Name = name
	return b
}

func TestParseEmpty(t *testing.T) {
	doc := ``

	objs, err := ParseMultidoc([]byte(doc), "test")
	if err != nil {
		t.Error(err)
	}
	if len(objs) != 0 {
		t.Errorf("expected empty set; got %#v", objs)
	}
}

func TestParseSome(t *testing.T) {
	docs := `---
kind: Deployment
metadata:
  name: b-deployment
  namespace: b-namespace
---
kind: Deployment
metadata:
  name: a-deployment
`
	objs, err := ParseMultidoc([]byte(docs), "test")
	if err != nil {
		t.Error(err)
	}

	objA := base("test", "Deployment", "", "a-deployment")
	objB := base("test", "Deployment", "b-namespace", "b-deployment")
	expected := map[string]resource.Resource{
		objA.ResourceID().String(): &Deployment{baseObject: objA},
		objB.ResourceID().String(): &Deployment{baseObject: objB},
	}

	for id, obj := range expected {
		// Remove the bytes, so we can compare
		if !reflect.DeepEqual(obj, debyte(objs[id])) {
			t.Errorf("At %+v expected:\n%#v\ngot:\n%#v", id, obj, objs[id])
		}
	}
}

func TestParseSomeWithComment(t *testing.T) {
	docs := `# some random comment
---
kind: Deployment
metadata:
  name: b-deployment
  namespace: b-namespace
---
kind: Deployment
metadata:
  name: a-deployment
`
	objs, err := ParseMultidoc([]byte(docs), "test")
	if err != nil {
		t.Error(err)
	}

	objA := base("test", "Deployment", "", "a-deployment")
	objB := base("test", "Deployment", "b-namespace", "b-deployment")
	expected := map[string]resource.Resource{
		objA.ResourceID().String(): &Deployment{baseObject: objA},
		objB.ResourceID().String(): &Deployment{baseObject: objB},
	}
	expectedL := len(expected)

	if len(objs) != expectedL {
		t.Errorf("expected %d objects from yaml source\n%s\n, got result: %d", expectedL, docs, len(objs))
	}

	for id, obj := range expected {
		// Remove the bytes, so we can compare
		if !reflect.DeepEqual(obj, debyte(objs[id])) {
			t.Errorf("At %+v expected:\n%#v\ngot:\n%#v", id, obj, objs[id])
		}
	}
}

func TestParseSomeLong(t *testing.T) {
	doc := `---
kind: ConfigMap
metadata:
  name: bigmap
data:
  bigdata: |
`
	buffer := bytes.NewBufferString(doc)
	line := "    The quick brown fox jumps over the lazy dog.\n"
	for buffer.Len()+len(line) < 1024*1024 {
		buffer.WriteString(line)
	}

	_, err := ParseMultidoc(buffer.Bytes(), "test")
	if err != nil {
		t.Error(err)
	}
}

func debyte(r resource.Resource) resource.Resource {
	if res, ok := r.(interface {
		debyte()
	}); ok {
		res.debyte()
	}
	return r
}

func TestLoadSome(t *testing.T) {
	dir, cleanup := testfiles.TempDir(t)
	defer cleanup()
	if err := testfiles.WriteTestFiles(dir); err != nil {
		t.Fatal(err)
	}
	objs, err := Load(dir, dir)
	if err != nil {
		t.Error(err)
	}
	if len(objs) != len(testfiles.ServiceMap(dir)) {
		t.Errorf("expected %d objects from %d files, got result:\n%#v", len(testfiles.ServiceMap(dir)), len(testfiles.Files), objs)
	}
}

func TestChartTracker(t *testing.T) {
	dir, cleanup := testfiles.TempDir(t)
	defer cleanup()
	if err := testfiles.WriteTestFiles(dir); err != nil {
		t.Fatal(err)
	}

	ct, err := newChartTracker(dir)
	if err != nil {
		t.Fatal(err)
	}

	noncharts := []string{"garbage", "locked-service-deploy.yaml",
		"test", "test/test-service-deploy.yaml"}
	for _, f := range noncharts {
		fq := filepath.Join(dir, f)
		if ct.isChart(fq) {
			t.Errorf("%q thought to be a chart", f)
		}
		if f == "garbage" {
			continue
		}
		if m, err := Load(dir, fq); err != nil || len(m) == 0 {
			t.Errorf("Load returned 0 objs, err=%v", err)
		}
	}
	if !ct.isChart(filepath.Join(dir, "charts/nginx")) {
		t.Errorf("charts/nginx not recognized as chart")
	}
	if !ct.inChart(filepath.Join(dir, "charts/nginx/Chart.yaml")) {
		t.Errorf("charts/nginx/Chart.yaml not recognized as in chart")
	}

	chartfiles := []string{"charts",
		"charts/nginx",
		"charts/nginx/Chart.yaml",
		"charts/nginx/values.yaml",
		"charts/nginx/templates/deployment.yaml",
	}
	for _, f := range chartfiles {
		fq := filepath.Join(dir, f)
		if m, err := Load(dir, fq); err != nil || len(m) != 0 {
			t.Errorf("%q not ignored as a chart should be", f)
		}
	}

}
