package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/walrus-rclone/mvp/backend"
)

// InteractiveSetup guides the user through configuration
func InteractiveSetup() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("🐋 Welcome to Walrus Storage CLI Setup")
	fmt.Println("=======================================")
	fmt.Println()
	fmt.Println("This wizard will help you configure Walrus storage access.")
	fmt.Println()

	// Network selection
	network := promptNetwork(reader)

	// Set endpoints based on network
	var aggregatorURL, publisherURL string
	switch network {
	case "testnet":
		aggregatorURL = "https://aggregator.walrus-testnet.walrus.space"
		publisherURL = "https://publisher.walrus-testnet.walrus.space"
	case "mainnet":
		aggregatorURL = "https://aggregator.walrus.space"
		publisherURL = "https://publisher.walrus.space"
	case "custom":
		aggregatorURL = promptString(reader, "Enter Aggregator URL", "https://aggregator.walrus-testnet.walrus.space")
		publisherURL = promptString(reader, "Enter Publisher URL", "https://publisher.walrus-testnet.walrus.space")
	}

	// Storage duration
	epochs := promptEpochs(reader)

	// Wallet configuration
	privateKey := promptWallet(reader, network)

	// Create configuration
	config := &backend.Config{
		Walrus: backend.WalrusConfig{
			AggregatorURL: aggregatorURL,
			PublisherURL:  publisherURL,
			Epochs:        epochs,
			Wallet: backend.WalletConfig{
				PrivateKey: privateKey,
			},
		},
	}

	// Get config path
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	configPath := filepath.Join(home, ".walrus-rclone", "config.yaml")

	// Confirm before saving
	fmt.Println()
	fmt.Println("Configuration Summary:")
	fmt.Println("======================")
	fmt.Printf("Network:       %s\n", network)
	fmt.Printf("Aggregator:    %s\n", aggregatorURL)
	fmt.Printf("Publisher:     %s\n", publisherURL)
	fmt.Printf("Default Epochs: %d\n", epochs)
	if privateKey != "" {
		fmt.Printf("Wallet:        Configured (%s...)\n", privateKey[:10])
	} else {
		fmt.Printf("Wallet:        Not configured\n")
	}
	fmt.Printf("Config Path:   %s\n", configPath)
	fmt.Println()

	if !promptConfirm(reader, "Save this configuration?") {
		fmt.Println("Setup cancelled.")
		return nil
	}

	// Save configuration
	if err := backend.SaveConfig(config, configPath); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Println()
	fmt.Println("✅ Configuration saved successfully!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("-----------")

	if network == "testnet" {
		fmt.Println("1. Get test WAL tokens:")
		fmt.Println("   Visit: https://discord.gg/walrus")
		fmt.Println("   Request tokens in #testnet-faucet")
		fmt.Println()
	}

	fmt.Println("2. Upload your first file:")
	fmt.Println("   walrus-cli upload myfile.pdf")
	fmt.Println()
	fmt.Println("3. List stored files:")
	fmt.Println("   walrus-cli list")
	fmt.Println()
	fmt.Println("Happy storing! 🚀")

	return nil
}

func promptNetwork(reader *bufio.Reader) string {
	fmt.Println("Select Network:")
	fmt.Println("1) Testnet (recommended for testing)")
	fmt.Println("2) Mainnet (requires real WAL tokens)")
	fmt.Println("3) Custom endpoints")
	fmt.Print("\nChoice [1]: ")

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	switch input {
	case "2":
		fmt.Println()
		fmt.Println("⚠️  Mainnet Warning:")
		fmt.Println("   - Requires real WAL tokens")
		fmt.Println("   - Storage costs real money")
		fmt.Println("   - Transactions are permanent")
		fmt.Println()
		if !promptConfirm(reader, "Continue with Mainnet?") {
			return promptNetwork(reader)
		}
		return "mainnet"
	case "3":
		return "custom"
	default:
		return "testnet"
	}
}

func promptEpochs(reader *bufio.Reader) int {
	fmt.Println()
	fmt.Println("Default Storage Duration:")
	fmt.Println("(1 epoch ≈ 2 weeks on testnet, longer on mainnet)")
	fmt.Print("Number of epochs [5]: ")

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return 5
	}

	var epochs int
	if _, err := fmt.Sscanf(input, "%d", &epochs); err != nil || epochs <= 0 {
		fmt.Println("Invalid input, using default: 5")
		return 5
	}

	return epochs
}

func promptWallet(reader *bufio.Reader, network string) string {
	fmt.Println()
	fmt.Println("Wallet Configuration (Optional):")

	if network == "testnet" {
		fmt.Println("Note: For testnet, you can skip this and add it later.")
	} else {
		fmt.Println("Note: Required for mainnet operations.")
	}

	fmt.Println()
	fmt.Println("Enter your Sui wallet private key")
	fmt.Println("(starts with 'suiprivkey1...' or leave empty to skip)")
	fmt.Print("Private key: ")

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		if network == "mainnet" {
			fmt.Println()
			fmt.Println("⚠️  Warning: Mainnet requires a wallet for operations.")
			if !promptConfirm(reader, "Continue without wallet?") {
				return promptWallet(reader, network)
			}
		}
		return ""
	}

	// Basic validation
	if !strings.HasPrefix(input, "suiprivkey1") {
		fmt.Println()
		fmt.Println("⚠️  Invalid private key format.")
		fmt.Println("   Sui private keys start with 'suiprivkey1'")
		fmt.Println()
		if promptConfirm(reader, "Try again?") {
			return promptWallet(reader, network)
		}
		return ""
	}

	// Mask the key for security
	fmt.Printf("✓ Private key accepted: %s...%s\n", input[:15], input[len(input)-4:])

	return input
}

func promptString(reader *bufio.Reader, prompt, defaultVal string) string {
	fmt.Printf("%s [%s]: ", prompt, defaultVal)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultVal
	}
	return input
}

func promptConfirm(reader *bufio.Reader, prompt string) bool {
	fmt.Printf("%s (y/n): ", prompt)
	input, _ := reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))
	return input == "y" || input == "yes"
}