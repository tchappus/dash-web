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
}

func getGitData() (*[7][]int64, error) {
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
	}

	weekDays := [7][]int64{}

	for _, week := range result.Data.User.ContributionsCollection.ContributionCalendar.Weeks {
		for y, day := range week.CommitDays {
			weekDays[y] = append(weekDays[y], day.CommitCount)
		}
	}
	return &weekDays, nil
}

func viewHandler(w http.ResponseWriter, r *http.Request) {
	weekDays, err := getGitData()

	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}

	p := &Page{
		CommitWeekDays: weekDays,
	}
	t, _ := template.ParseFiles("template.html")
	t.Execute(w, p)
}

func main() {
	http.HandleFunc("/dash/", viewHandler)
	log.Fatal(http.ListenAndServe(":8082", nil))
}
