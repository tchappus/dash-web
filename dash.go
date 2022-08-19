package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

type QueryVariables struct {
	Username string `json:"userName"`
}

type GitRequest struct {
	Query     string          `json:"query"`
	Variables *QueryVariables `json:"variables"`
}

type GitResponse struct {
	Data struct {
		User struct {
			ContributionsCollection struct {
				ContributionCalendar struct {
					TotalContributions int64        `json:"totalContributions"`
					Weeks              []CommitWeek `json:"weeks"`
				} `json:"contributionCalendar"`
			} `json:"contributionsCollection"`
		} `json:"user"`
	} `json:"data"`
}

type CommitWeek struct {
	CommitDays []struct {
		CommitCount int64  `json:"contributionCount"`
		Date        string `json:"date"`
	} `json:"contributionDays"`
}

type Page struct {
	CommitWeekDays *[7][]int64
	CommitRatio    float64
	Temp           float32
}

type WeatherResponse struct {
	Data struct {
		Timelines []struct {
			Timestep  string `json:"timestep"`
			StartTime string `json:"startTime"`
			EndTime   string `json:"endTime"`
			Intervals []struct {
				StartTime string `json:"startTime"`
				Values    struct {
					Temperature float32 `json:"temperature"`
				} `json:"values"`
			} `json:"intervals"`
		} `json:"timelines"`
	} `json:"data"`
}

func getGitData() (*GitResponse, error) {
	query := `
	query($userName:String!) {
		user(login: $userName){
		  contributionsCollection {
			contributionCalendar {
			  totalContributions
			  weeks {
				contributionDays {
				  contributionCount
				  date
				}
			  }
			}
		  }
		}
	  }
	`

	queryVars := &QueryVariables{
		Username: "tchappus",
	}

	gitRequest := &GitRequest{
		Query:     query,
		Variables: queryVars,
	}

	reqBuf := new(bytes.Buffer)
	json.NewEncoder(reqBuf).Encode(gitRequest)

	req, _ := http.NewRequest("POST", "https://api.github.com/graphql", reqBuf)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("GITHUB_TOKEN")))

	client := &http.Client{}
	res, e := client.Do(req)

	if e != nil {
		fmt.Println(e)
		return nil, e
	}

	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	var result GitResponse
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println(e)
		return nil, err
	}

	return &result, nil
}

func getWeatherData() (*WeatherResponse, error) {
	request := struct {
		Location  [2]float64 `json:"location"`
		Fields    [1]string  `json:"fields"`
		Units     string     `json:"units"`
		StartTime string     `json:"startTime"`
		EndTime   string     `json:"endTime"`
		Timezone  string     `json:"timezone"`
	}{
		Location:  [...]float64{45.5245773, -73.596708},
		Fields:    [...]string{"temperature"},
		Units:     "metric",
		StartTime: "now",
		EndTime:   "nowPlus6h",
		Timezone:  "America/Montreal",
	}

	payloadBuf := new(bytes.Buffer)
	json.NewEncoder(payloadBuf).Encode(request)

	req, _ := http.NewRequest("POST", fmt.Sprintf("https://api.tomorrow.io/v4/timelines?apikey=%s", os.Getenv("TOMORROW_IO_TOKEN")), payloadBuf)

	client := &http.Client{}
	res, e := client.Do(req)

	if e != nil {
		fmt.Println("error while doing tomorrow.io rest call", e)
		return nil, e
	}

	defer res.Body.Close()

	fmt.Println("Response from tomorrow.io:", res.Status)

	body, _ := ioutil.ReadAll(res.Body)

	var result WeatherResponse
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println(e)
		return nil, err
	}

	return &result, nil
}

func generateWeekDays(res *GitResponse) (*[7][]int64, int64) {
	weekDays := [7][]int64{}
	maxCommits := int64(0)
	weeks := res.Data.User.ContributionsCollection.ContributionCalendar.Weeks
	lastFiftyTwoWeeks := weeks[len(weeks)-52:]
	for _, week := range lastFiftyTwoWeeks {
		for y, day := range week.CommitDays {

			if day.CommitCount > maxCommits {
				maxCommits = day.CommitCount
			}

			weekDays[y] = append(weekDays[y], day.CommitCount)
		}
	}
	return &weekDays, int64(maxCommits)
}

func viewHandler(w http.ResponseWriter, r *http.Request) {
	gitRes, err := getGitData()

	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	weekDays, maxCommits := generateWeekDays(gitRes)
	commitRatio := 100 / maxCommits

	weatherRes, err := getWeatherData()

	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	p := &Page{
		CommitWeekDays: weekDays,
		CommitRatio:    float64(commitRatio),
		Temp:           weatherRes.Data.Timelines[0].Intervals[0].Values.Temperature,
	}

	generateWebpage(p, &w)
}

func generateWebpage(p *Page, w *http.ResponseWriter) {

	t, err := template.New("template.gohtml").Funcs(template.FuncMap{
		"commitOpacity": func(commits int64) int64 {
			return int64(p.CommitRatio) * commits
		},
	}).ParseFiles("template.gohtml")

	if err != nil {
		fmt.Printf(err.Error())
		return
	}
	err = t.Execute(*w, p)

	if err != nil {
		fmt.Printf(err.Error())
		return
	}
}

func main() {
	http.HandleFunc("/dash/", viewHandler)
	log.Fatal(http.ListenAndServe(":8082", nil))
}
