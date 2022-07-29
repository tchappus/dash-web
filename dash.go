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

func generateWeekDays(res *GitResponse) (*[7][]int64, int64) {
	weekDays := [7][]int64{}
	maxCommits := int64(0)
	for _, week := range res.Data.User.ContributionsCollection.ContributionCalendar.Weeks {
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
	res, err := getGitData()

	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	generateWebpage(res, &w)
}

func testHandler(w http.ResponseWriter, r *http.Request) {

	file, _ := os.ReadFile("test.json")
	var res *GitResponse
	err := json.Unmarshal(file, &res)

	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}

	generateWebpage(res, &w)
}

func generateWebpage(res *GitResponse, w *http.ResponseWriter) {
	weekDays, maxCommits := generateWeekDays(res)

	ratio := 100 / maxCommits

	p := &Page{
		CommitWeekDays: weekDays,
	}
	t, err := template.New("template.html").Funcs(template.FuncMap{
		"commitOpacity": func(commits int64) int64 {
			return ratio * commits
		},
	}).ParseFiles("template.html")

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
	http.HandleFunc("/test/", testHandler)
	log.Fatal(http.ListenAndServe(":8082", nil))
}

// func main() {
// 	vars := &QueryVariables{
// 		Username: "foo",
// 	}

// 	req := GitRequest{
// 		Variables: vars,
// 		Query:     "",
// 	}

// 	req.Variables.Username = "bar"

// 	fmt.Println(vars.Username)

// }
