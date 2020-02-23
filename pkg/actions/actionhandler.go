package actions

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	petname "github.com/dustinkirkland/golang-petname"
	"github.com/gorilla/mux"
	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var Data [][]byte

type Message struct {
	Test string
}

type K8SReq struct {
	Namespaces      string `json:"namespaces"`
	Deployments     string `json:"deployments"`
	Pods            string `json:"pods"`
	RequestedCPU    string `json:"reqCPU"`
	RequestedMemory string `json:"reqMem"`
	LimitCPU        string `json:"limCPU"`
	LimitMemory     string `json:"limMem"`
	LoadProfile     string `json:"profile"`
}

func ActionHandler(rw http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)

	switch vars["action"] {

	case "malloc20mb":
		log.Printf("Allocating 20mb to existing %d Mb", len(Data)/2048*2)

		for i := 0; i < 1024*20; i++ {
			kb := make([]byte, 1024)
			rand.Read(kb)
			Data = append(Data, kb)
		}

		res := fmt.Sprintf("Size now: %d Mb", len(Data)/2048*2)

		rw.Write([]byte(res))

	case "livenessoff":
		//RespondToHealth = false

		rw.Write([]byte("Letting /health time out from now on"))

	case "k8sload":

		rand.Seed(time.Now().UnixNano())

		nsb, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		cNs := string(nsb)

		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			log.Printf("%s", err)
			return
		}

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			log.Printf("%s", err)
			return
		}

		k8sreq := &K8SReq{}
		err = json.Unmarshal(b, k8sreq)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		config, err := rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
		// creates the clientset
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}

		dps, err := strconv.Atoi(k8sreq.Deployments)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		pdsI, err := strconv.Atoi(k8sreq.Pods)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
		pds := int32(pdsI)

		// Limits and requests in podspec.
		limits := make(v1.ResourceList)
		if len(k8sreq.LimitCPU) > 0 && k8sreq.LimitCPU != "0" {
			limits["cpu"], _ = resource.ParseQuantity(k8sreq.LimitCPU)
		}
		if len(k8sreq.LimitMemory) > 0 && k8sreq.LimitMemory != "0" {
			limits["memory"], _ = resource.ParseQuantity(k8sreq.LimitMemory)
		}

		requests := make(v1.ResourceList)
		if len(k8sreq.RequestedCPU) > 0 && k8sreq.RequestedCPU != "0" {
			requests["cpu"], _ = resource.ParseQuantity(k8sreq.RequestedCPU)
		}
		if len(k8sreq.RequestedMemory) > 0 && k8sreq.RequestedMemory != "0" {
			requests["memory"], _ = resource.ParseQuantity(k8sreq.RequestedMemory)
		}

		// Load Profile
		command := []string{}

		switch k8sreq.LoadProfile {
		case "none":
			command = []string{"/kubez"}
		case "cpu":
			command = []string{"/load100cpu"}
		case "mem100":
			command = []string{"/loadmem100m"}
		case "mem200":
			command = []string{"/loadmem200m"}
		case "mem2000":
			command = []string{"/loadmem2g"}

		}

		for d := 0; d < dps; d++ {

			dpNmae := fmt.Sprintf("kl-%s", petname.Generate(3, "-"))

			clientset.AppsV1().Deployments(cNs).Create(&appv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: dpNmae,
				},
				Spec: appv1.DeploymentSpec{
					Replicas: &pds,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": dpNmae},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": dpNmae}},
						Spec: v1.PodSpec{
							Containers: []v1.Container{

								{
									Resources: v1.ResourceRequirements{
										Limits:   limits,
										Requests: requests,
									},
									Name:  "kubez",
									Image: "docker.io/middlewaregruppen/kubez",
									Ports: []v1.ContainerPort{
										{ContainerPort: 3000},
									},
									Command: command,
								},
							},
						},
					},
				},
			})

		}

	case "fileinfo":
		nofiles := 0
		var size int64
		var files []string
		filepath.Walk("/", func(path string, info os.FileInfo, err error) error {

			if strings.HasPrefix("/dev", path) {
				return nil
			}
			if strings.HasPrefix("/proc", path) {
				return nil
			}

			if err != nil {
				return nil
			}
			files = append(files, info.Name())
			nofiles++
			size = size + info.Size()
			return nil
		})

		res := fmt.Sprintf("Found %d files. Size: %d Mb", nofiles, size/1024/1024)

		rw.Write([]byte(res))

	case "log100":
		lines := 100
		start := time.Now()
		for i := 0; i < lines; i++ {
			log.Printf("Logging a lot: %d ", i)

		}
		d := time.Since(start)
		res := fmt.Sprintf("Logged %d lines in %.2f seconds", lines, d.Seconds())

		rw.Write([]byte(res))

	case "log1000":
		lines := 1000
		start := time.Now()
		for i := 0; i < lines; i++ {
			log.Printf("Logging a lot: %d ", i)

		}
		d := time.Since(start)
		res := fmt.Sprintf("Logged %d lines in %.2f seconds", lines, d.Seconds())

		rw.Write([]byte(res))

	case "log10000":
		lines := 10000
		start := time.Now()
		for i := 0; i < lines; i++ {
			log.Printf("Logging a lot: %d ", i)

		}
		d := time.Since(start)
		res := fmt.Sprintf("Logged %d lines in %.2f seconds", lines, d.Seconds())

		rw.Write([]byte(res))

	case "cpusmall":
		const testBytes = `{ "Test": "value" }`
		iter := int64(700000)
		start := time.Now()
		p := &Message{}
		for i := int64(1); i < iter; i++ {
			json.NewDecoder(strings.NewReader(testBytes)).Decode(p)
		}
		d := time.Since(start)
		res := fmt.Sprintf("[small]. Took %.2f seconds", d.Seconds())
		rw.Write([]byte(res))

	case "cpumedium":
		const testBytes = `{ "Test": "value" }`
		iter := int64(3000000)
		start := time.Now()
		p := &Message{}
		for i := int64(1); i < iter; i++ {
			json.NewDecoder(strings.NewReader(testBytes)).Decode(p)
		}
		d := time.Since(start)
		res := fmt.Sprintf("Done: %.2f s", d.Seconds())
		rw.Write([]byte(res))

	case "cpularge":
		const testBytes = `{ "Test": "value" }`
		iter := int64(8000000)
		start := time.Now()
		p := &Message{}
		for i := int64(1); i < iter; i++ {
			json.NewDecoder(strings.NewReader(testBytes)).Decode(p)
		}
		d := time.Since(start)
		res := fmt.Sprintf("[large]. Took %.2f seconds", d.Seconds())
		rw.Write([]byte(res))

		/*case "metrics-increase":
			opsProcessed.Inc()

			rw.Write([]byte("clicks has been increased"))

		case "metrics-gauge-10":
			gauge.Set(10)
			rw.Write([]byte("ata_request_load set to 10"))

		case "metrics-gauge-50":
			gauge.Set(50)
			rw.Write([]byte("ata_request_load set to 50"))

		case "metrics-gauge-90":
			gauge.Set(90)
			rw.Write([]byte("ata_request_load set to 90"))

		case "tracing-flow1":
			span, ctx := opentracing.StartSpanFromContext(r.Context(), "awesome_business_function")
			defer span.Finish()

			time.Sleep(200 * time.Millisecond)

			if !BusinessFunction(ctx) {

				rw.Write([]byte("☠️☠️☠️ Request failed! 🤬 "))

			} else {
				rw.Write([]byte(" 🥳 Request successful! 👻 "))
			}
		*/
	}

}
