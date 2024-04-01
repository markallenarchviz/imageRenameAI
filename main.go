package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/widgets"
)

type OCRResponse struct {
	OCRExitCode                  int    `json:"OCRExitCode"`
	IsErroredOnProcessing        bool   `json:"IsErroredOnProcessing"`
	ErrorMessage                 string `json:"ErrorMessage"`
	ProcessingTimeInMilliseconds string `json:"ProcessingTimeInMilliseconds"`
	ParsedResults                []struct {
		TextOverlay struct {
			Lines []interface{} `json:"Lines"`
			Words []struct {
				WordText string `json:"WordText"`
				Left     int    `json:"Left"`
				Top      int    `json:"Top"`
				Height   int    `json:"Height"`
				Width    int    `json:"Width"`
			} `json:"Words"`
		} `json:"TextOverlay"`
		TextOrientation   string `json:"TextOrientation"`
		FileParseExitCode int    `json:"FileParseExitCode"`
		ParsedText        string `json:"ParsedText"`
		ErrorMessage      string `json:"ErrorMessage"`
		ErrorDetails      string `json:"ErrorDetails"`
	} `json:"ParsedResults"`
	ProcessingError string `json:"ProcessingError"`
}

func ocrSpaceFile(filename string, overlay bool, apiKey string, language string, ocrEngine string) (*OCRResponse, error) {
	url := "https://api.ocr.space/parse/image"

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	_ = writer.WriteField("apikey", apiKey)
	_ = writer.WriteField("language", language)
	_ = writer.WriteField("OCREngine", ocrEngine)
	_ = writer.WriteField("isOverlayRequired", fmt.Sprintf("%t", overlay))

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return nil, err
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var ocrResponse OCRResponse
	err = json.Unmarshal(body, &ocrResponse)
	if err != nil {
		return nil, err
	}

	return &ocrResponse, nil
}

func extractTimePattern(text string) string {
	re := regexp.MustCompile(`\b\d{2}:\d{2}:\d{2}\b`)
	timePattern := re.FindString(text)
	timePattern = strings.ReplaceAll(timePattern, ":", ".")
	return timePattern
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func main() {
	app := widgets.NewQApplication(len(os.Args), os.Args)

	mainWindow := widgets.NewQMainWindow(nil, 0)
	mainWindow.SetWindowTitle("Total Vision(Codevasf) - ImageRename")
	mainWindow.SetFixedSize2(400, 100) // Set window size

	centralWidget := widgets.NewQWidget(nil, 0)
	centralWidgetLayout := widgets.NewQVBoxLayout()
	centralWidget.SetLayout(centralWidgetLayout)
	mainWindow.SetCentralWidget(centralWidget)

	folderPathLabel := widgets.NewQLabel2("", nil, 0)
	centralWidgetLayout.AddWidget(folderPathLabel, 0, 0)

	selectFolderBtn := widgets.NewQPushButton2("Selecionar Pasta", nil)
	centralWidgetLayout.AddWidget(selectFolderBtn, 0, 0)

	progressBar := widgets.NewQProgressBar(nil)
	progressBar.SetMinimum(0)
	progressBar.SetMaximum(100)
	progressBar.SetValue(0)
	centralWidgetLayout.AddWidget(progressBar, 0, 0)

	selectFolderBtn.ConnectClicked(func(bool) {
		folderPath := widgets.QFileDialog_GetExistingDirectory(mainWindow, "Select Folder", core.QDir_HomePath(), widgets.QFileDialog__ShowDirsOnly)
		if folderPath != "" {
			folderPathLabel.SetText(folderPath)

			processImages(folderPath, progressBar, mainWindow)
		}
	})

	mainWindow.Show()

	app.Exec()
}

func processImages(inputFolder string, progressBar *widgets.QProgressBar, mainWindow *widgets.QMainWindow) {
	outputFolder := filepath.Join(inputFolder, "Fotos Renomeadas")

	totalImages := 0
	filepath.Walk(inputFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(info.Name()) == ".jpg" {
			totalImages++
		}
		return nil
	})

	processedImages := 0
	filepath.Walk(inputFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(info.Name()) == ".jpg" {
			ocrResponse, err := ocrSpaceFile(path, false, "K81163354788957", "pol", "2")
			if err != nil {
				return err
			}

			if len(ocrResponse.ParsedResults) > 0 {
				timePattern := extractTimePattern(ocrResponse.ParsedResults[0].ParsedText)

				if _, err := os.Stat(outputFolder); os.IsNotExist(err) {
					err = os.MkdirAll(outputFolder, 0755)
					if err != nil {
						return err
					}
				}

				newFileName := filepath.Join(outputFolder, timePattern+".jpg")
				err := copyFile(path, newFileName)
				if err != nil {
					return err
				}
			}

			processedImages++
			progress := float64(processedImages) / float64(totalImages) * 100
			progressBar.SetValue(int(progress))

			if processedImages == totalImages {
				mainWindow.Close()
			}
		}
		return nil
	})
}
