package analysis

import (
	"github.com/ananagame/Olivia/slice"
	"github.com/ananagame/Olivia/triggers"
	"github.com/fxsjy/gonn/gonn"
	"github.com/go-redis/redis"
	"math/rand"
	"sort"
	"time"
	"regexp"
)

// Initialize the user's context cache
var cache = make(map[string]string)

type Sentence struct {
	Content string
}

type Result struct {
	Tag   string
	Value float64
}

const letters = "[a-zàâçéèêëîïôùûœæA-ZÀÂÇÉÈÊËÎÏÔÙÛŒÆ0-9-]"

// Returns an array of tokenized words
func (sentence Sentence) Tokenize() []string {
	regex, err := regexp.Compile(
		"(\\d+(\\.\\d+)?\\s*%)|(" + letters + "')|(" + letters + "+)|([^" + letters + "\\s])",
	)

	if err != nil {
		panic(err)
	}

	words := regex.FindAllString(sentence.Content, -1)
	var stemmedWords []string

	for _, word := range words {
		stemmedWords = append(stemmedWords, Word{word}.Stem())
	}

	return stemmedWords
}

// Retrieves all the intents words and returns the bag of words of the Sentence content
func (sentence Sentence) WordsBag(words []string) (bag []float64) {
	for _, word := range words {
		// Append 1 if the patternWords contains the actual word, else 0
		var valueToAppend float64 = 0
		if slice.Contains(sentence.Tokenize(), word) {
			valueToAppend = 1
		}

		bag = append(bag, valueToAppend)
	}

	return bag
}

// Classify the sentence with the model
func (sentence Sentence) PredictTag(n gonn.NeuralNetwork) string {
	words, classes, _ := Organize()

	// Predict with the model
	predict := n.Forward(sentence.WordsBag(words))

	// Enumerate the results with the intent tags
	var resultsTag []Result
	for i, result := range predict {
		resultsTag = append(resultsTag, Result{classes[i], result})
	}

	// Sort the results in ascending order
	sort.Slice(resultsTag, func(i, j int) bool {
		return resultsTag[i].Value > resultsTag[j].Value
	})

	return resultsTag[0].Tag
}

// Returns the human readable response
func RandomizeResponse(entry string, tag string, userId string) string {
	// Iterate all the json intents
	for _, intent := range SerializeIntents() {
		if intent.Tag != tag {
			continue
		}

		if intent.Context != "" && cache[userId] != intent.Context {
			return "Je ne comprends pas :("
		}

		cache[userId] = intent.Tag

		response := intent.Responses[0]
		// Return a random response if there are more than one
		if len(intent.Responses) > 1 {
			response = intent.Responses[rand.Intn(len(intent.Responses))]
		}

		// Apply triggers
		for _, trigger := range triggers.RegisteredTriggers(entry, response) {
			response = trigger.ReplaceContent()
		}

		return response
	}

	// Error
	return "Désolé, je n'ai pas compris"
}

// Respond with the cache or the model
func (sentence Sentence) Calculate(client redis.Client, network gonn.NeuralNetwork, userId string) string {
	tag, err := client.Get(sentence.Content).Result()

	// If the sentence isn't in the redis database
	if err == redis.Nil {
		tag = sentence.PredictTag(network)
		client.Set(sentence.Content, tag, 2*time.Minute)
	} else if err != nil {
		panic(err)
	}

	return RandomizeResponse(sentence.Content, tag, userId)
}
