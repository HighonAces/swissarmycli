package k8s

import (
	"bufio"
	"context"
	"fmt"
	"github.com/HighonAces/swissarmycli/internal/k8s/common"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"strconv"
	"strings"
)

// printDecodedSecret is a helper function to neatly print the contents of a secret.
func printDecodedSecret(secret *v1.Secret) {
	if len(secret.Data) == 0 {
		fmt.Printf("Secret '%s' in namespace '%s' contains no data.\n", secret.Name, secret.Namespace)
		return
	}

	fmt.Printf("\n--- Decoded Secret Data: '%s' (Namespace: %s) ---\n", secret.Name, secret.Namespace)
	for key, value := range secret.Data {
		// The `client-go` library automatically decodes the secret data for us.
		// The `value` here is a raw byte slice (`[]byte`) of the already-decoded data.
		// We just need to cast it to a string to print it.
		fmt.Printf("%s: %s\n", key, string(value))
	}
	fmt.Println("----------------------------------------------------")
}

func RevealSecret(secretName, namespace string) error {
	clientset, err := common.GetKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}
	// --- Case 1: Namespace is provided via the -n/--namespace flag ---
	if namespace != "" {
		fmt.Printf("Fetching secret '%s' from the namespace '%s'...\n", secretName, namespace)

		secret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get secret '%s' in namespace '%s': %w", secretName, namespace, err)
		}
		printDecodedSecret(secret)
		return nil
	}

	// --- Case 2: No namespace provided; search all namespaces ---
	fmt.Printf("No namespace provided. Searching for secret '%s' across all namespaces...\n", secretName)
	allSecrets, err := clientset.CoreV1().Secrets("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list secrets in all namespaces: %w", err)
	}

	// Filter the list to find secrets with the matching name.
	var foundSecrets []v1.Secret
	for _, secret := range allSecrets.Items {
		if secret.Name == secretName {
			foundSecrets = append(foundSecrets, secret)
		}
	}

	// --- Handle the search results ---
	switch len(foundSecrets) {
	case 0:
		// No secrets with that name were found anywhere.
		return fmt.Errorf("secret '%s' not found in any namespace", secretName)

	case 1:
		// Exactly one match was found, so we can print it directly.
		secret := foundSecrets[0]
		fmt.Printf("Found one match in namespace '%s'.\n", secret.Namespace)
		printDecodedSecret(&secret)

	default:
		// Multiple matches found, so we need to ask the user which one they want.
		fmt.Printf("Found multiple secrets named '%s'. Please choose one:\n", secretName)
		for i, secret := range foundSecrets {
			fmt.Printf("[%d] %s\n", i+1, secret.Namespace)
		}

		// Create a reader to get user input from the console.
		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Print("Enter number: ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)

			choice, err := strconv.Atoi(input)
			if err != nil || choice < 1 || choice > len(foundSecrets) {
				fmt.Printf("Invalid input. Please enter a number between 1 and %d.\n", len(foundSecrets))
				continue // Ask again if the input is not a valid number in the range.
			}

			// Use the user's choice to select the correct secret.
			selectedSecret := foundSecrets[choice-1]
			printDecodedSecret(&selectedSecret)
			break // Exit the loop after a valid choice.
		}
	}

	return nil
}
