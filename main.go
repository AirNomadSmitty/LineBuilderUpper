package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"

	"github.com/draffensperger/golp"
)

const salariesCSV = "DKSalariesWeek16.csv"
const salaryCap = 50000
const uniques = 1

var positionMaxes = map[string]float64{
	"QB":   1,
	"RB":   3,
	"WR":   4,
	"TE":   2,
	"DST":  1,
	"Flex": 7,
}
var positionMins = map[string]float64{
	"QB":   1,
	"RB":   2,
	"WR":   3,
	"TE":   1,
	"DST":  1,
	"Flex": 7,
}

type Player struct {
	Name       string
	Position   string
	Salary     int
	Team       string
	Projection float64
	ID         int
}

type Lineup struct {
	Score         float64
	Salary        int
	PlayerIndexes []int
	WRs           []*Player
	RBs           []*Player
	TE            *Player
	QB            *Player
	Flex          *Player
	DST           *Player
}

func (lineup *Lineup) ForceAddPlayer(player *Player) {
	lineup.PlayerIndexes = append(lineup.PlayerIndexes, player.ID)
	switch player.Position {
	case "QB":
		lineup.QB = player
		break
	case "RB":
		if len(lineup.RBs) == 2 {
			lineup.Flex = player
		} else {
			lineup.RBs = append(lineup.RBs, player)
		}
		break
	case "WR":
		if len(lineup.WRs) == 3 {
			lineup.Flex = player
		} else {
			lineup.WRs = append(lineup.WRs, player)
		}
		break
	case "TE":
		if lineup.TE != nil {
			lineup.Flex = player
		} else {
			lineup.TE = player
		}
		break
	case "DST":
		lineup.DST = player
	}
}

func main() {
	// lp := golp.NewLP(0, 3)

	// lp.AddConstraint([]float64{110.0, 30.0, 1.0}, golp.LE, 4000.0)
	// lp.AddConstraint([]float64{1.0, 1.0, 1.0}, golp.LE, 75.0)
	// lp.SetObjFn([]float64{143.0, 60.0, 100.0})
	// lp.SetMaximize()

	// lp.Solve()
	// vars := lp.Variables()
	// fmt.Printf("Plant %.3f acres of barley\n", vars[0])
	// fmt.Printf("And  %.3f acres of wheat\n", vars[1])
	// fmt.Printf("And  %.3f acres of good stuff\n", vars[2])
	// fmt.Printf("For optimal profit of $%.2f\n", lp.Objective())

	// playersByPosition := map[string][]*Player{
	// 	"QB": []*Player{
	// 		&Player{"Aaron Rodgers", "QB", 5000, "GB", 5.0, 1.0}},
	// 	"RB":  []*Player{&Player{"Christian McCaffrey", "RB", 10000, "CAR", 5.0, 1.0}, &Player{"Alvin Kamara", "RB", 5000, "CAR", 5.0, 1.0}, &Player{"Saquon Barkley", "RB", 5000, "CAR", 5.0, 1.0}},
	// 	"WR":  []*Player{&Player{"Curtis Samuel", "WR", 5000, "CAR", 5.0, 1.0}, &Player{"DJ Moore", "WR", 5000, "CAR", 5.0, 1.0}, &Player{"Jarius Wright", "WR", 5000, "CAR", 5.0, 1.0}},
	// 	"TE":  []*Player{&Player{"Greg Olsen", "TE", 5000, "CAR", 5.0, 1.0}, &Player{"Ian Thomas", "TE", 5000, "CAR", 5.0, 1.0}},
	// 	"DST": []*Player{&Player{"Carolina", "DST", 5000, "CAR", 5.0, 1.0}},
	// }

	playersByPosition, count := parseFile(salariesCSV)

	var (
		budgetConstraint []float64
		scoreObjective   []float64
	)

	variablePlayerMap := make(map[int]*Player)
	lp := golp.NewLP(0, count)

	i := 0
	var flexConstraint []golp.Entry
	for pos, positionPlayers := range playersByPosition {
		var positionConstraint []golp.Entry
		for _, player := range positionPlayers {
			player.ID = i
			variablePlayerMap[i] = player
			budgetConstraint = append(budgetConstraint, float64(player.Salary))
			scoreObjective = append(scoreObjective, player.Projection)
			positionConstraint = append(positionConstraint, golp.Entry{i, 1})
			if pos == "RB" || pos == "WR" || pos == "TE" {
				flexConstraint = append(flexConstraint, golp.Entry{i, 1})
			}
			i++
		}
		if positionMaxes[pos] == positionMins[pos] {
			lp.AddConstraintSparse(positionConstraint, golp.EQ, positionMaxes[pos])
		} else {
			lp.AddConstraintSparse(positionConstraint, golp.LE, positionMaxes[pos])
			lp.AddConstraintSparse(positionConstraint, golp.GE, positionMins[pos])
		}
	}
	lp.AddConstraint(budgetConstraint, golp.LE, salaryCap)
	lp.AddConstraintSparse(flexConstraint, golp.EQ, positionMaxes["Flex"])
	for i := range budgetConstraint {
		lp.SetBinary(i, true)
	}
	lp.SetObjFn(scoreObjective)
	lp.SetMaximize()

	var lineups []*Lineup
	for i := 0; i < 1; i++ {
		lp.Solve()
		vars := lp.Variables()
		lineup := makeLineupFromVars(vars, variablePlayerMap)
		lineups = append(lineups, lineup)
		uniqueConstraint := makeUniqueConstraintFromLineup(lineup)
		lp.AddConstraintSparse(uniqueConstraint, golp.LE, float64(len(lineup.PlayerIndexes)-uniques))
	}

	PrettyPrint(lineups)
}

func makeUniqueConstraintFromLineup(lineup *Lineup) []golp.Entry {
	var uniqueConstraint []golp.Entry
	for _, index := range lineup.PlayerIndexes {
		uniqueConstraint = append(uniqueConstraint, golp.Entry{index, 1})
	}

	return uniqueConstraint
}

func makeLineupFromVars(vars []float64, variablePlayerMap map[int]*Player) *Lineup {
	lineup := &Lineup{}
	for index, val := range vars {
		if val == 1 {
			player := variablePlayerMap[index]
			lineup.Salary += player.Salary
			lineup.Score += player.Projection
			lineup.ForceAddPlayer(player)
		}
	}

	return lineup
}

func parseFile(filename string) (map[string][]*Player, int) {
	csvFile, _ := os.Open(filename)
	reader := csv.NewReader(bufio.NewReader(csvFile))

	positionPlayers := make(map[string][]*Player)
	var players []*Player
	i := 0
	for {
		line, error := reader.Read()
		if error == io.EOF {
			break
		} else if error != nil {
			log.Fatal(error)
		}
		/*
			0 = name
			1 = projection
			2 = position
			3 = team
			4 = salary
		*/
		position := line[2]
		salary, _ := strconv.Atoi(line[4])
		projection, _ := strconv.ParseFloat(line[1], 64)
		if projection < 5 {
			continue
		}
		player := &Player{Name: line[0], Position: line[2], Salary: salary, Team: line[3], Projection: projection}
		positionPlayers[position] = append(positionPlayers[position], player)
		players = append(players, player)
		i++
	}

	return positionPlayers, i
}

func PrettyPrint(i interface{}) {
	s, _ := json.MarshalIndent(i, "", "\t")
	fmt.Println(string(s))
}
