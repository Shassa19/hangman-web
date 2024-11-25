package main

import (
	"bufio"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type Game struct {
	Word          string
	RevealedWord  []rune
	AttemptsLeft  int
	UsedLetters   map[rune]bool
	HangmanStages []string
	Message       string
}

var (
	mots   []string
	etapes []string
	mutex  sync.Mutex
	games  = make(map[string]*Game)
)

func init() {
	mots = chargerMots("words.txt")
	etapes = chargerPendu("hangman.txt")
}

func chargerMots(fichier string) []string {
	file, err := os.Open(fichier)
	if err != nil {
		fmt.Println("Erreur lors de l'ouverture du fichier :", err)
		os.Exit(1)
	}
	defer file.Close()

	var mots []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		mots = append(mots, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Erreur lors de la lecture du fichier :", err)
		os.Exit(1)
	}

	return mots
}

func chargerPendu(fichier string) []string {
	file, err := os.Open(fichier)
	if err != nil {
		fmt.Println("Erreur lors de l'ouverture du fichier :", err)
		os.Exit(1)
	}
	defer file.Close()

	var stages []string
	var stage string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			stages = append(stages, stage)
			stage = ""
		} else {
			stage += line + "\n"
		}
	}
	if stage != "" {
		stages = append(stages, stage)
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Erreur lors de la lecture du fichier :", err)
		os.Exit(1)
	}

	return stages
}

func choisirMot(mots []string) string {
	rand.Seed(time.Now().UnixNano())
	return mots[rand.Intn(len(mots))]
}

func nouvellePartie() *Game {
	word := choisirMot(mots)
	return &Game{
		Word:          word,
		RevealedWord:  revelerLettres(word, len(word)/2-1),
		AttemptsLeft:  10,
		UsedLetters:   make(map[rune]bool),
		HangmanStages: etapes,
		Message:       "",
	}
}

func revelerLettres(word string, n int) []rune {
	revealed := make([]rune, len(word))
	for i := range revealed {
		revealed[i] = '_'
	}
	indices := rand.Perm(len(word))[:n]
	for _, idx := range indices {
		revealed[idx] = rune(word[idx])
	}
	return revealed
}

func getGame(r *http.Request) *Game {
	mutex.Lock()
	defer mutex.Unlock()

	sessionID := "default"
	if cookie, err := r.Cookie("game-session"); err == nil {
		sessionID = cookie.Value
	}
	if game, exists := games[sessionID]; exists {
		return game
	}

	newGame := nouvellePartie()
	games[sessionID] = newGame
	return newGame
}

func saveGame(w http.ResponseWriter, r *http.Request, game *Game) {
	mutex.Lock()
	defer mutex.Unlock()

	sessionID := "default"
	if cookie, err := r.Cookie("game-session"); err == nil {
		sessionID = cookie.Value
	}

	games[sessionID] = game
}

func gameHandler(w http.ResponseWriter, r *http.Request) {
	game := getGame(r)
	game.Message = ""

	if r.Method == http.MethodPost {
		letter := r.FormValue("lettre")
		if len(letter) == 1 {
			letterRune := rune(letter[0])
			if !game.UsedLetters[letterRune] {
				game.UsedLetters[letterRune] = true
				if strings.ContainsRune(game.Word, letterRune) {
					for i, char := range game.Word {
						if char == letterRune {
							game.RevealedWord[i] = letterRune
						}
					}
					game.Message = "Bonne lettre !"
				} else {
					game.AttemptsLeft--
					game.Message = "Lettre incorrecte."
				}
			} else {
				game.Message = "Vous avez déjà essayé cette lettre."
			}
		} else {
			game.Message = "Veuillez entrer une seule lettre."
		}
	}

	lettersTried := []string{}
	for letter := range game.UsedLetters {
		lettersTried = append(lettersTried, string(letter))
	}
	lettersTriedString := strings.Join(lettersTried, ", ")

	win := strings.Compare(game.Word, string(game.RevealedWord)) == 0
	lose := game.AttemptsLeft <= 0

	tmpl, err := template.ParseFiles("template/hangman.tmpl")
	if err != nil {
		http.Error(w, "Erreur lors du chargement du template HTML", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"EtatMot":             afficherMotRevele(game.RevealedWord),
		"TentativesRestantes": game.AttemptsLeft,
		"Perdu":               lose,
		"Gagne":               win,
		"Mot":                 game.Word,
		"LettresEssayees":     lettersTriedString,
		"Message":             game.Message,
		"HangmanStage":        game.HangmanStages[10-game.AttemptsLeft],
	}

	saveGame(w, r, game)
	tmpl.Execute(w, data)
}

func afficherMotRevele(revealedWord []rune) string {
	return strings.Join(strings.Split(string(revealedWord), ""), " ")
}

func main() {
	http.Handle("/style/", http.StripPrefix("/style/", http.FileServer(http.Dir("style"))))
	http.HandleFunc("/", gameHandler)

	fmt.Println("Le serveur est en cours d'exécution sur http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
