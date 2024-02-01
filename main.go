package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"gopkg.in/yaml.v2"
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func main() {
	ctx := context.Background()
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	spreadsheetId := "1OxUGr5qJ835LgPa45J93Iu1adKGNVdPTWgT7QMnKdww"

	// readRange := "Class Data!A2:E"
	// resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, readRange).Do()
	// if err != nil {
	// 	log.Fatalf("Unable to retrieve data from sheet: %v", err)
	// }

	// if len(resp.Values) == 0 {
	// 	fmt.Println("No data found.")
	// } else {
	// 	fmt.Println("Name, Major:")
	// 	for _, row := range resp.Values {
	// 		// Print columns A and E, which correspond to indices 0 and 4.
	// 		fmt.Printf("%s, %s\n", row[0], row[4])
	// 	}
	// }

	carData := readData()

	// The A1 notation of the values to update
	writeRange := "Sheet2!A1:E1" // this points to the first row

	sheetRange := "Sheet2!A%d:E%d"
	// The new values to apply to the spreadsheet
	values := []*sheets.ValueRange{
		{
			Range:  writeRange,
			Values: [][]interface{}{{"make name", "id", "", "model name", "id"}},
		},
		// {
		// 	Range:  "Sheet2!A2:D2",
		// 	Values: [][]interface{}{{"Row 2 Col 1", "Row 2 Col 2", "Row 2 Col 3", "Row 2 Col 4"}},
		// },
	}

	for i, carMake := range carData.CarMake {
		modelLength := 0
		if i > 0 {
			for k := 0; k < i; k++ {
				modelLength += len(carData.CarMake[k].Models)
			}
		}
		for j, carModel := range carMake.Models {
			carMakeName := ""
			carMakeID := ""
			if j == 0 {
				carMakeName = carMake.Name
				carMakeID = carMake.ID
			}
			start := 1
			values = append(values, &sheets.ValueRange{
				Range:  fmt.Sprintf(sheetRange, start+i+j+modelLength+2, start+i+j+modelLength+2),
				Values: [][]interface{}{{carMakeName, carMakeID, "", carModel.Name, carModel.ID}},
			})
		}
	}

	for i, valueRange := range values {
		_, err := srv.Spreadsheets.Values.Update(spreadsheetId, valueRange.Range, &sheets.ValueRange{
			MajorDimension: "ROWS",
			Values:         valueRange.Values,
		}).ValueInputOption("RAW").Do()
		if i%10 == 0 {
			time.Sleep(2 * time.Second)
		}

		if err != nil {
			log.Fatalf("Unable to set data. %v", err)
		}
	}

}

type CarData struct {
	CarMake []CarMake
}
type CarMake struct {
	ID     string `yaml: "id"`
	Name   string `yaml: "name"`
	Models []Model
}
type CarModel struct {
	Values []Model `yaml:"values"`
	MakeID string  `yaml:"parent_id"`
}

type Model struct {
	ID   string `yaml: "id"`
	Name string `yaml: "name"`
}

type YAMLData struct {
	Make  []CarMake  `yaml:"make"`
	Model []CarModel `yaml:"model"`
}

func readData() CarData {
	// Specify the path to your YAML file
	filePath := "vehicles.yaml"

	// Read the YAML file
	yamlFile, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Error reading YAML file: %v", err)
	}

	// Create a Config struct to unmarshal the YAML data into
	var config YAMLData

	// Unmarshal the YAML data into the Config struct
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		log.Fatalf("Error unmarshalling YAML data: %v", err)
	}

	carModelData := make(map[string][]Model) // key => make_id, values => models
	for _, carModel := range config.Model {
		carModelData[carModel.MakeID] = carModel.Values
	}

	var carData CarData
	for _, data := range config.Make {
		data.Models = carModelData[data.ID]
		carData.CarMake = append(carData.CarMake, data)
	}

	//carDataJson, _ := json.Marshal(carData)
	//fmt.Println("car data <<", string(carDataJson))

	return carData
}
