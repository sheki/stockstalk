package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vic3lord/stocks"
)

type config struct {
	Investments []investment             `json:"investments"`
	History     map[string][]performance `json:"history"` // history is keyed by the symbol
}

type performance struct {
	Symbol           string    `json:"symbol"`
	Price            float64   `json:"price"`
	CompoundInterest float64   `json:"compound_interest"`
	Date             time.Time `json:"date"`
}

type investment struct {
	Symbol string    `json:"symbol"`
	Date   time.Time `json:"date"`
	Total  float64   `json:"total"`
	Units  float64   `json:"units"`
}

func perr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
}

func main() {
	var add = flag.String("add", "", "set an investment as \"symbol,date(mm/dd/yy),total(float64),units(float64)\" takes priority")
	var config = flag.String("config", "config.json", "file to set config at")
	flag.Parse()

	if *add != "" {
		perr(addInvestment(*add, *config))
		return
	}

	perr(analysis(*config))
}

const secondsPerYear = 365.25 * 24 * 60 * 60 // leap year hack

func currentRate(i investment, price float64) float64 {
	principal := i.Total / i.Units
	d := time.Now().Sub(i.Date).Seconds() / secondsPerYear
	r := 100 * (math.Pow(price/principal, 1/d) - 1)
	return r
}

func analysis(confFile string) error {
	conf, err := parseConfig(confFile)
	if err != nil {
		return err
	}
	if conf.History == nil {
		conf.History = make(map[string][]performance)
	}
	for _, i := range conf.Investments {
		stock, err := stocks.GetQuote(i.Symbol)
		if err != nil {
			return err
		}
		p, err := stock.GetPrice()
		if err != nil {
			return err
		}
		r := currentRate(i, p)
		perf := performance{
			Symbol:           i.Symbol,
			Date:             time.Now(),
			CompoundInterest: r,
			Price:            p,
		}
		hPerf := conf.History[i.Symbol]
		hPerf = append(hPerf, perf)
		conf.History[i.Symbol] = hPerf
	}
	err = writeConfig(confFile, conf)
	if err != nil {
		return err
	}

	printAnalysis(conf)
	return nil
}

const humanDate = "02-Jan-06"

func printAnalysis(conf config) {
	for _, v := range conf.Investments {
		history := conf.History[v.Symbol]
		fmt.Printf("%s %f %s\n", v.Symbol, v.Total, v.Date.Format(humanDate))
		if history == nil {
			continue
		}
		for i := len(history) - 1; i >= 0; i-- {
			fmt.Printf("%s %.2f\n", history[i].Date.Format(humanDate), history[i].CompoundInterest)
		}
	}
}

func parseConfig(file string) (config, error) {
	f, err := os.Open(file)
	if err != nil {
		if os.IsNotExist(err) {
			return config{}, nil
		}
		return config{}, err
	}
	defer f.Close()
	var c config
	err = json.NewDecoder(f).Decode(&c)
	return c, err
}

func writeConfig(file string, conf config) error {
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(conf)
}

const mmddyy = "1/2/2006"

func parseInvestmentLine(iStr string) (investment, error) {
	arr := strings.Split(iStr, ",")
	if len(arr) != 4 {
		return investment{}, errors.New("investment line format incorrect")
	}
	t, err := time.Parse(mmddyy, arr[1])
	if err != nil {
		return investment{}, err
	}

	total, err := strconv.ParseFloat(arr[2], 64)
	if err != nil {
		return investment{}, err
	}
	units, err := strconv.ParseFloat(arr[3], 64)
	return investment{
		Symbol: arr[0],
		Date:   t,
		Total:  total,
		Units:  units,
	}, err
}

func addInvestment(iStr string, confFile string) error {
	conf, err := parseConfig(confFile)
	if err != nil {
		return err
	}
	i, err := parseInvestmentLine(iStr)
	if err != nil {
		return err
	}
	conf.Investments = append(conf.Investments, i)
	return writeConfig(confFile, conf)
}
