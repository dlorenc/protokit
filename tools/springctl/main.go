package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pwittrock/protokit/tools/springctl/discovery"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

func main() {
	name := strings.ToLower(filepath.Base(os.Args[0]))
	var result []runtime.Object
	var err error

	i := os.Getenv("KUSTOMIZE_PLUGIN_CONFIG_STRING")
	if len(os.Args) > 1 {
		var b []byte
		b, err = ioutil.ReadFile(os.Args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error %v", err)
			os.Exit(1)
		}
		i = string(b)
	}

	switch name {
	case "springcloudservice":
		doSpringCloudService()
	case "springclouddiscoveryservice":
		result, err = discovery.DoSpringCloudDiscoveryService(i)
	case "springctl":
		doSpringCtl()
	default:
		err = fmt.Errorf("type %s unsupported\n", name)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error generating %s: %v", os.Getenv("KUSTOMIZE_PLUGIN_CONFIG_STRING"), err)
		os.Exit(1)
	}

	var out string
	for i := range result {
		o, err := yaml.Marshal(result[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error generating %s: %v", os.Getenv("KUSTOMIZE_PLUGIN_CONFIG_STRING"), err)
			os.Exit(1)
		}
		out = fmt.Sprintf("%s---\n%s", out, o)
	}

	fmt.Println(out)
}

func doSpringCloudService() {
	fmt.Printf("service: %s\n", os.Args[0])

}

func doSpringCtl() {
	fmt.Printf("ctl: %s\n", os.Args[0])
}
