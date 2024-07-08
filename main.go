package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/time/rate"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type CustomResourceStatus struct {
	Status  string           `json:"status"`
	Details []UnhealthyChild `json:"details,omitempty"`
	Message string           `json:"message,omitempty"`
}

type ResourceStatus struct {
	CustomResourceStatus CustomResourceStatus
	Timestamp            time.Time
	ConsecHealthyChecks  int
	ConsecFailedChecks   int
}

var (
	statusCache           = make(map[string]ResourceStatus)
	statusCacheMu         sync.Mutex
	rateLimiter           *rate.Limiter
	checkInterval         = 25 * time.Second
	consecHealthy         = 4
	consecFailed          = 3
	limiterRate           = 10
	limiterBurst          = 20
	increaseIntervalValue = 15 * time.Second
	readyCheckInterval    = 60 * time.Second
	initialDelay          = 10 * time.Second // Default initial delay
)

func init() {
	if value, exists := os.LookupEnv("CHECK_INTERVAL"); exists {
		if parsedValue, err := time.ParseDuration(value); err == nil {
			checkInterval = parsedValue
		}
	}

	if value, exists := os.LookupEnv("CONSEC_HEALTHY"); exists {
		if parsedValue, err := strconv.Atoi(value); err == nil {
			consecHealthy = parsedValue
		}
	}

	if value, exists := os.LookupEnv("CONSEC_FAILED"); exists {
		if parsedValue, err := strconv.Atoi(value); err == nil {
			consecFailed = parsedValue
		}
	}

	if value, exists := os.LookupEnv("LIMITER_RATE"); exists {
		if parsedValue, err := strconv.Atoi(value); err == nil {
			limiterRate = parsedValue
		}
	}

	if value, exists := os.LookupEnv("LIMITER_BURST"); exists {
		if parsedValue, err := strconv.Atoi(value); err == nil {
			limiterBurst = parsedValue
		}
	}

	if value, exists := os.LookupEnv("INCREASE_INTERVAL_VALUE"); exists {
		if parsedValue, err := time.ParseDuration(value); err == nil {
			increaseIntervalValue = parsedValue
		}
	}

	if value, exists := os.LookupEnv("READY_CHECK_INTERVAL"); exists {
		if parsedValue, err := time.ParseDuration(value); err == nil {
			readyCheckInterval = parsedValue
		}
	}

	if value, exists := os.LookupEnv("INITIAL_DELAY"); exists {
		if parsedValue, err := time.ParseDuration(value); err == nil {
			initialDelay = parsedValue
		}
	}
}

func main() {
	flag.Parse()

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("[ERROR] Failed to create in-cluster config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("[ERROR] Failed to create clientset: %v", err)
	}

	rateLimiter = rate.NewLimiter(rate.Limit(limiterRate), limiterBurst)

	r := mux.NewRouter()
	r.HandleFunc("/healthz", healthzHandler).Methods("GET")
	r.HandleFunc("/health/{crdGroup}/{crdVersion}/{crdPlural}/{namespace}/{name}", healthHandler(clientset)).Methods("GET")
	r.HandleFunc("/reset/{crdGroup}/{crdVersion}/{crdPlural}/{namespace}/{name}", resetHandler).Methods("POST")

	log.Println("[INFO] Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func healthHandler(clientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		vars := mux.Vars(r)
		crdGroup := vars["crdGroup"]
		crdVersion := vars["crdVersion"]
		crdPlural := vars["crdPlural"]
		namespace := vars["namespace"]
		name := vars["name"]
		labelSelector := r.URL.Query().Get("labelSelector")
		annotationSelector := r.URL.Query().Get("annotationSelector")

		key := fmt.Sprintf("%s/%s/%s/%s/%s", crdGroup, crdVersion, crdPlural, namespace, name)

		statusCacheMu.Lock()
		status, exists := statusCache[key]
		statusCacheMu.Unlock()

		if exists && time.Since(status.Timestamp) < checkInterval {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(status.CustomResourceStatus)
			log.Printf("[INFO] Returning cached response for resource: %s/%s in namespace %s of kind %s/%s", name, crdPlural, namespace, crdGroup, crdVersion)
			return
		}

		go monitorHealth(clientset, key, crdGroup, crdVersion, crdPlural, namespace, name, labelSelector, annotationSelector, initialDelay)

		if exists {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(status.CustomResourceStatus)
			log.Printf("[INFO] Returning cached response for resource: %s/%s in namespace %s of kind %s/%s", name, crdPlural, namespace, crdGroup, crdVersion)
		} else {
			w.WriteHeader(http.StatusAccepted)
			response := CustomResourceStatus{Status: "deploying", Message: "Initial check in progress"}
			json.NewEncoder(w).Encode(response)
			log.Printf("[INFO] Initial response for resource: %s/%s in namespace %s of kind %s/%s", name, crdPlural, namespace, crdGroup, crdVersion)
		}

		log.Printf("[INFO] Request processing time: %v", time.Since(startTime))
	}
}

func resetHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	crdGroup := vars["crdGroup"]
	crdVersion := vars["crdVersion"]
	crdPlural := vars["crdPlural"]
	namespace := vars["namespace"]
	name := vars["name"]

	key := fmt.Sprintf("%s/%s/%s/%s/%s", crdGroup, crdVersion, crdPlural, namespace, name)

	statusCacheMu.Lock()
	defer statusCacheMu.Unlock()

	if status, exists := statusCache[key]; exists {
		status.ConsecHealthyChecks = 0
		status.ConsecFailedChecks = 0
		statusCache[key] = status
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Counters reset for resource: %s", key)))
		log.Printf("[INFO] Counters reset for resource: %s", key)
	} else {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(fmt.Sprintf("Resource not found: %s", key)))
		log.Printf("[INFO] Resource not found: %s", key)
	}
}

func monitorHealth(clientset *kubernetes.Clientset, key, crdGroup, crdVersion, crdPlural, namespace, name, labelSelector, annotationSelector string, initialDelay time.Duration) {
	time.Sleep(initialDelay) // Adding initial delay before starting the checks

	consecutiveHealthyChecks := 0
	consecutiveNotFoundChecks := 0
	failedCheckInterval := checkInterval
	lastReadyCheck := time.Now()

	for {
		if err := rateLimiter.Wait(context.Background()); err != nil {
			log.Printf("[ERROR] Rate limiter error: %v", err)
			return
		}

		newStatus, err := checkHealth(clientset, crdGroup, crdVersion, crdPlural, namespace, name, labelSelector, annotationSelector)
		if err != nil {
			log.Printf("[ERROR] Error checking health for %s: %v", key, err)
			consecutiveNotFoundChecks++
			if consecutiveNotFoundChecks >= consecFailed {
				statusCacheMu.Lock()
				statusCache[key] = ResourceStatus{
					CustomResourceStatus: CustomResourceStatus{Status: "failed", Message: "Resource not found after multiple checks"},
					Timestamp:            time.Now(),
				}
				statusCacheMu.Unlock()
				return
			}
			failedCheckInterval += increaseIntervalValue
		} else {
			consecutiveNotFoundChecks = 0
			statusCacheMu.Lock()
			statusCache[key] = ResourceStatus{
				CustomResourceStatus: newStatus,
				Timestamp:            time.Now(),
			}
			statusCacheMu.Unlock()

			if newStatus.Status == "ready" {
				consecutiveHealthyChecks++
				if time.Since(lastReadyCheck) >= readyCheckInterval {
					log.Printf("[INFO] Rechecking ready resource %s after ready check interval.", key)
					lastReadyCheck = time.Now()
				}
			} else {
				consecutiveHealthyChecks = 0
			}

			if newStatus.Status == "failed" {
				consecFailed = 0
			}

			if consecutiveHealthyChecks >= consecHealthy {
				log.Printf("[INFO] Resource %s has been ready for %d consecutive checks. Stopping further checks.", key, consecHealthy)
				return
			}
			failedCheckInterval = checkInterval
		}

		time.Sleep(failedCheckInterval)
	}
}

func checkHealth(clientset *kubernetes.Clientset, crdGroup, crdVersion, crdPlural, namespace, name, labelSelector, annotationSelector string) (CustomResourceStatus, error) {
	ctx := context.TODO()

	log.Printf("[INFO] Fetching custom resource: crdGroup=%s, crdVersion=%s, crdPlural=%s, namespace=%s, name=%s", crdGroup, crdVersion, crdPlural, namespace, name)
	customResource, err := clientset.RESTClient().
		Get().
		AbsPath("/apis", crdGroup, crdVersion, "namespaces", namespace, crdPlural, name).
		DoRaw(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[INFO] Resource not found: %v", err)
			return CustomResourceStatus{Status: "deploying", Message: "Waiting for resource in kubernetes"}, nil
		}
		return CustomResourceStatus{}, fmt.Errorf("[ERROR] Failed to get custom resource: %v", err)
	}

	var crMap map[string]interface{}
	if err := json.Unmarshal(customResource, &crMap); err != nil {
		return CustomResourceStatus{}, fmt.Errorf("[ERROR] Failed to unmarshal custom resource: %v", err)
	}

	prettyJSON, err := json.MarshalIndent(crMap, "", "  ")
	if err != nil {
		return CustomResourceStatus{}, fmt.Errorf("[ERROR] Failed to marshal pretty JSON: %v", err)
	}

	log.Printf("[INFO] Custom resource fetched: %s", string(prettyJSON))

	var crStatus map[string]interface{}
	log.Printf("[INFO] Unmarshaling custom resource status")
	err = json.Unmarshal(customResource, &crStatus)
	if err != nil {
		return CustomResourceStatus{}, fmt.Errorf("[ERROR] Failed to unmarshal custom resource: %v", err)
	}

	log.Printf("[INFO] Custom resource status: %+v", crStatus)

	var unhealthyChildren []UnhealthyChild
	overallStatus := "ready"
	isCompleteCheck := true

	checkers := []ResourceChecker{
		PodChecker{},
		JobChecker{},
		PVChecker{},
		PVCChecker{},
	}

	for _, checker := range checkers {
		log.Printf("[INFO] Running checker: %T for resource: %s/%s in namespace %s of kind %s/%s", checker, name, crdPlural, namespace, crdGroup, crdVersion)
		err = checker.Check(ctx, clientset, namespace, crdGroup, crdVersion, crdPlural, labelSelector, annotationSelector, &unhealthyChildren, &overallStatus)
		if err != nil {
			log.Printf("[ERROR] Error checking resource with checker %T for resource: %s/%s in namespace %s of kind %s/%s: %v", checker, name, crdPlural, namespace, crdGroup, crdVersion, err)
			isCompleteCheck = false
			return CustomResourceStatus{}, err
		}
	}

	// Ensure status is only "ready" or "failed" after all checks
	if isCompleteCheck {
		if len(unhealthyChildren) > 0 {
			overallStatus = "deploying"
		} else if overallStatus == "ready" {
			overallStatus = "ready"
		} else {
			overallStatus = "deploying"
		}
	} else {
		overallStatus = "deploying"
	}

	log.Printf("[INFO] Resource status: %s for resource: %s/%s in namespace %s of kind %s/%s", overallStatus, name, crdPlural, namespace, crdGroup, crdVersion)

	return CustomResourceStatus{Status: overallStatus, Details: unhealthyChildren}, nil
}

func matchAnnotations(resourceAnnotations map[string]string, annotationSelector string) bool {
	if annotationSelector == "" {
		return true
	}
	parts := strings.SplitN(annotationSelector, "=", 2)
	if len(parts) != 2 {
		return false
	}
	key, value := parts[0], parts[1]
	if v, ok := resourceAnnotations[key]; ok && v == value {
		return true
	}
	return false
}
