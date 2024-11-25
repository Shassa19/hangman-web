/*
package main

import (

	"fmt"
	"net/http"

)

	func main() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Welcome to my website!")
		})

		fs := http.FileServer(http.Dir("static/"))
		http.Handle("/static/", http.StripPrefix("/static/", fs))

		http.ListenAndServe(":80", nil)
	}
*/
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

// Structure du jeu
type Game struct {
	Word          string
	RevealedWord  []rune
	AttemptsLeft  int
	UsedLetters   map[rune]bool
	HangmanStages []string
}

// Variables globales
var (
	mots   []string
	etapes []string
	mutex  sync.Mutex
	games  = make(map[string]*Game) // Stockage des sessions de jeu
)

func init() {
	// Charger les mots et les étapes du pendu au démarrage
	mots = chargerMots("words.txt")
	etapes = chargerPendu("hangman.txt")
}

// Charger la liste des mots
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

// Charger les étapes du pendu
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

// Choisir un mot aléatoire
func choisirMot(mots []string) string {
	rand.Seed(time.Now().UnixNano())
	return mots[rand.Intn(len(mots))]
}

// Créer une nouvelle partie
func nouvellePartie() *Game {
	word := choisirMot(mots)
	return &Game{
		Word:          word,
		RevealedWord:  revelerLettres(word, len(word)/2-1),
		AttemptsLeft:  10,
		UsedLetters:   make(map[rune]bool),
		HangmanStages: etapes,
	}
}

// Révéler quelques lettres du mot
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

// Gérer la session de jeu
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

	// Créer une nouvelle session
	newGame := nouvellePartie()
	games[sessionID] = newGame
	return newGame
}

// Sauvegarder la session
func saveGame(w http.ResponseWriter, r *http.Request, game *Game) {
	mutex.Lock()
	defer mutex.Unlock()

	sessionID := "default"
	if cookie, err := r.Cookie("game-session"); err == nil {
		sessionID = cookie.Value
	}

	games[sessionID] = game
}

// Gérer les requêtes
func gameHandler(w http.ResponseWriter, r *http.Request) {
	game := getGame(r)

	// Gérer une soumission de lettre
	if r.Method == http.MethodPost {
		letter := r.FormValue("lettre") // Utiliser 'lettre' comme clé pour la lettre
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
				} else {
					game.AttemptsLeft--
				}
			}
		}
	}

	// Déterminer l'état du jeu (victoire ou défaite)
	win := strings.Compare(game.Word, string(game.RevealedWord)) == 0
	lose := game.AttemptsLeft <= 0

	// Charger le template HTML
	tmpl, err := template.ParseFiles("hangman.tmpl")
	if err != nil {
		http.Error(w, "Erreur lors du chargement du template HTML", http.StatusInternalServerError)
		return
	}

	// Rendu de la page avec les données du jeu
	data := map[string]interface{}{
		"EtatMot":             afficherMotRevele(game.RevealedWord),
		"TentativesRestantes": game.AttemptsLeft,
		"Perdu":               lose,
		"Gagne":               win,
		"Mot":                 game.Word,
		"LettresEssayees":     game.UsedLetters,
		"HangmanStage":        game.HangmanStages[10-game.AttemptsLeft],
	}

	saveGame(w, r, game) // Sauvegarder l'état du jeu
	tmpl.Execute(w, data)
}

// Afficher le mot révélé avec des espaces
func afficherMotRevele(revealedWord []rune) string {
	return strings.Join(strings.Split(string(revealedWord), ""), " ")
}

func main() {
	// Gérer les fichiers statiques (CSS, images, etc.)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Route pour le jeu
	http.HandleFunc("/", gameHandler)

	// Démarrer le serveur
	fmt.Println("Le serveur est en cours d'exécution sur http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
