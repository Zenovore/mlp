package delete

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gojek/mlp/api/pkg/gcs"
)

type deleteClient struct {
	Client     *http.Client
	GcsPackage gcs.GcsPackage
	Config     Config
}

type Config struct {
	TrackingURL string
}

type DeletePackage interface {
	DeleteExperiment(trackingURL string, idExperiment string, deleteArtifact bool)
	DeleteRun(trackingURL string, idRun string, delArtifact bool)
}

func NewDeleteClient(delClient *http.Client, gcspkg gcs.GcsPackage, config Config) *deleteClient {
	return &deleteClient{
		Client:     delClient,
		GcsPackage: gcspkg,
		Config:     config,
	}
}

type deleteExperimentRequest struct {
	ExperimentId string `json:"experiment_id" required:"true"`
}

type deleteRunRequest struct {
	RunId string `json:"run_id" required:"true"`
}

type deleteExperimentErrorResponse struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

type searchRunRequest struct {
	ExperimentId []string `json:"experiment_ids" required:"true"`
}

type tagRun struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
type dataRun struct {
	Tags []tagRun `json:"tags"`
}

type infoRun struct {
	RunId          string `json:"run_id"`
	ExperimentId   string `json:"experiment_id"`
	UserId         string `json:"user_id"`
	LifecycleStage string `json:"lifecycle_stage"`
	ArtifactURI    string `json:"artifact_uri"`
}
type runResponse struct {
	Info infoRun `json:"info"`
	Data dataRun `json:"data"`
}
type searchRunsResponse struct {
	RunsData []runResponse `json:"runs"`
}
type searchRunResponse struct {
	RunData runResponse `json:"run"`
}

func (dc *deleteClient) httpCall(method string, url string, headers map[string]string, body []byte, response interface{}) error {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := dc.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
		// Convert response body to Error Message struct
		var errMessage deleteExperimentErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errMessage); err != nil {
			return err
		}
		return fmt.Errorf(errMessage.Message)
	}

	if response != nil {
		if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
			return err
		}
	}

	return nil
}

func (dc *deleteClient) DeleteExperiment(idExperiment string, deleteArtifact bool) error {
	// Creating Input Format for Delete experiment
	input := deleteExperimentRequest{ExperimentId: idExperiment}
	// HIT Delete Experiment API
	delExpURL := fmt.Sprintf("%s/api/2.0/mlflow/experiments/delete", dc.Config.TrackingURL)

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	jsonInput, err := json.Marshal(input)
	if err != nil {
		return err
	}

	err = dc.httpCall("POST", delExpURL, headers, jsonInput, nil)
	if err != nil {
		return err
	}

	relatedRunId, err := dc.SearchRunForExperiment(idExperiment)
	if err != nil {
		return err
	}
	var deletedRunId []string
	var failDeletedRunId []string
	for _, run := range relatedRunId.RunsData {
		err = dc.DeleteRun(run.Info.RunId, false)
		if err != nil {
			failDeletedRunId = append(failDeletedRunId, run.Info.RunId)
			// return err
		} else {
			deletedRunId = append(deletedRunId, run.Info.RunId)
		}
	}
	fmt.Println(failDeletedRunId)
	fmt.Println(deletedRunId)

	if len(relatedRunId.RunsData) > 0 {
		path := relatedRunId.RunsData[0].Info.ArtifactURI[5:]
		splitPath := strings.SplitN(path, "/", 4)
		folderPath := strings.Join(splitPath[0:3], "/")
		// deleting folder
		err = dc.GcsPackage.DeleteArtifact(folderPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func (dc *deleteClient) SearchRunForExperiment(idExperiment string) (searchRunsResponse, error) {
	// HIT Delete Experiment API
	var responseObject searchRunsResponse

	searchRunURL := fmt.Sprintf("%s/api/2.0/mlflow/runs/search", dc.Config.TrackingURL)

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	input := searchRunRequest{ExperimentId: []string{idExperiment}}
	jsonInput, err := json.Marshal(input)
	if err != nil {
		return responseObject, err
	}

	err = dc.httpCall("POST", searchRunURL, headers, jsonInput, &responseObject)
	if err != nil {
		return responseObject, err
	}

	return responseObject, nil
}

func (dc *deleteClient) SearchRunData(idRun string) (searchRunResponse, error) {
	// Creating Input Format for Delete experiment
	var runResponse searchRunResponse
	getRunURL := fmt.Sprintf("%s/api/2.0/mlflow/runs/get?run_id=%s", dc.Config.TrackingURL, idRun)

	err := dc.httpCall("GET", getRunURL, nil, nil, &runResponse)
	if err != nil {
		return runResponse, err
	}
	return runResponse, nil
}

func (dc *deleteClient) DeleteRun(idRun string, delArtifact bool) error {
	// Creating Input Format for Delete run
	input := deleteRunRequest{RunId: idRun}
	// HIT Delete Run API
	delRunURL := fmt.Sprintf("%s/api/2.0/mlflow/runs/delete", dc.Config.TrackingURL)

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	jsonInput, err := json.Marshal(input)
	if err != nil {
		return err
	}

	err = dc.httpCall("POST", delRunURL, headers, jsonInput, nil)
	if err != nil {
		return err
	}

	if delArtifact {
		runDetail, err := dc.SearchRunData(idRun)
		if err != nil {
			return err
		}
		fmt.Println(runDetail)
		err = dc.GcsPackage.DeleteArtifact(runDetail.RunData.Info.ArtifactURI[5:])
		if err != nil {
			return nil
		}

	}
	return nil
}
