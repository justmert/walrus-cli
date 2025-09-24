package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/justmert/walrus-cli/backend"
)

var (
	// Additional colors for modern.go (using Sprint for consistency with cobra.go)
	greenBold   = color.New(color.FgGreen, color.Bold).SprintFunc()
	yellowBold  = color.New(color.FgYellow, color.Bold).SprintFunc()
	redBold     = color.New(color.FgRed, color.Bold).SprintFunc()
	magentaBold = color.New(color.FgMagenta, color.Bold).SprintFunc()
)

// ModernInteractiveSetup provides a modern, colorized setup experience
func ModernInteractiveSetup() error {
	// Welcome banner
	fmt.Println()
	fmt.Println(cyanBold("Welcome to Walrus Storage CLI"))
	fmt.Println(strings.Repeat("=", 40))
	fmt.Println()

	fmt.Println("This wizard will guide you through setting up Walrus storage access.")
	fmt.Println()

	// Network selection with arrow keys
	network := ""
	networkPrompt := &survey.Select{
		Message: "Select your preferred network:",
		Options: []string{
			"Testnet (Free, for testing)",
			"Mainnet (Requires real WAL tokens)",
			"Custom endpoints",
		},
		Default: "Testnet (Free, for testing)",
	}

	var networkChoice string
	if err := survey.AskOne(networkPrompt, &networkChoice); err != nil {
		return fmt.Errorf("network selection failed: %w", err)
	}

	switch {
	case strings.Contains(networkChoice, "Testnet"):
		network = "testnet"
		fmt.Println(green("Selected Testnet - Perfect for getting started!"))
	case strings.Contains(networkChoice, "Mainnet"):
		network = "mainnet"
		fmt.Println(yellow("Selected Mainnet - Real tokens required!"))

		// Mainnet warning
		fmt.Println()
		fmt.Println(redBold("MAINNET WARNING:"))
		fmt.Println(red("   • Requires real WAL tokens"))
		fmt.Println(red("   • Storage costs real money"))
		fmt.Println(red("   • All transactions are permanent"))
		fmt.Println()

		confirm := false
		confirmPrompt := &survey.Confirm{
			Message: "Do you want to continue with Mainnet?",
			Default: false,
		}
		if err := survey.AskOne(confirmPrompt, &confirm); err != nil {
			return err
		}
		if !confirm {
			return ModernInteractiveSetup() // Restart
		}
	default:
		network = "custom"
		fmt.Println(blue("Selected Custom - You'll configure endpoints manually"))
	}

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
		// Custom endpoint prompts
		aggregatorPrompt := &survey.Input{
			Message: "Aggregator URL:",
			Default: "https://aggregator.walrus-testnet.walrus.space",
		}
		survey.AskOne(aggregatorPrompt, &aggregatorURL)

		publisherPrompt := &survey.Input{
			Message: "Publisher URL:",
			Default: "https://publisher.walrus-testnet.walrus.space",
		}
		survey.AskOne(publisherPrompt, &publisherURL)
	}

	fmt.Println()

	// Storage duration
	epochs := 0
	epochsPrompt := &survey.Select{
		Message: "Default storage duration:",
		Options: []string{
			"1 epoch (~2 weeks) - Short term",
			"5 epochs (~10 weeks) - Recommended",
			"10 epochs (~20 weeks) - Long term",
			"Custom duration",
		},
		Default: "5 epochs (~10 weeks) - Recommended",
	}

	var epochsChoice string
	if err := survey.AskOne(epochsPrompt, &epochsChoice); err != nil {
		return err
	}

	switch {
	case strings.Contains(epochsChoice, "1 epoch"):
		epochs = 1
	case strings.Contains(epochsChoice, "5 epochs"):
		epochs = 5
	case strings.Contains(epochsChoice, "10 epochs"):
		epochs = 10
	default:
		// Custom epochs
		customEpochs := 5
		customPrompt := &survey.Input{
			Message: "Enter number of epochs:",
			Default: "5",
		}
		var customStr string
		survey.AskOne(customPrompt, &customStr)
		fmt.Sscanf(customStr, "%d", &customEpochs)
		epochs = customEpochs
	}

	fmt.Printf(green("Storage duration: %d epochs\n"), epochs)
	fmt.Println()

	// Wallet configuration
	var privateKey string
	if network == "testnet" {
		fmt.Println(blue("Wallet Setup (Optional for Testnet)"))
		fmt.Println()
		fmt.Println("Note: You can skip this for testnet and add it later.")
	} else {
		fmt.Println(blue("Wallet Setup (Required for Mainnet)"))
		fmt.Println()
		fmt.Println(red("Note: Mainnet operations require a funded wallet."))
	}

	walletOptions := []string{
		"Enter private key now",
		"Skip for now (configure later)",
	}

	if network == "mainnet" {
		walletOptions = []string{
			"Enter private key now",
		}
	}

	walletPrompt := &survey.Select{
		Message: "Wallet configuration:",
		Options: walletOptions,
		Default: walletOptions[0],
	}

	var walletChoice string
	if err := survey.AskOne(walletPrompt, &walletChoice); err != nil {
		return err
	}

	if strings.Contains(walletChoice, "Enter private key") {
		keyPrompt := &survey.Password{
			Message: "Enter your Sui private key (starts with 'suiprivkey1'):",
		}

		if err := survey.AskOne(keyPrompt, &privateKey); err != nil {
			return err
		}

		// Validate private key
		if privateKey != "" && !strings.HasPrefix(privateKey, "suiprivkey1") {
			fmt.Println(red("Error: Invalid private key format!"))
			fmt.Println("   Sui private keys must start with 'suiprivkey1'")

			retry := false
			retryPrompt := &survey.Confirm{
				Message: "Would you like to try again?",
				Default: true,
			}
			survey.AskOne(retryPrompt, &retry)

			if retry {
				return ModernInteractiveSetup() // Restart
			}
			privateKey = ""
		} else if privateKey != "" {
			fmt.Printf(green("Private key accepted: %s...%s\n"), privateKey[:15], privateKey[len(privateKey)-4:])
		}
	}

	// Configuration summary
	fmt.Println()
	fmt.Println(cyanBold("Configuration Summary"))
	fmt.Println(strings.Repeat("-", 30))
	fmt.Printf("Network:        %s\n", getNetworkDisplay(network))
	fmt.Printf("Aggregator:     %s\n", aggregatorURL)
	fmt.Printf("Publisher:      %s\n", publisherURL)
	fmt.Printf("Default Epochs: %d\n", epochs)

	if privateKey != "" {
		fmt.Printf("Wallet:         %s\n", green("Configured"))
	} else {
		fmt.Printf("Wallet:         %s\n", yellow("Not configured"))
	}

	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".walrus-rclone", "config.yaml")
	fmt.Printf("Config Path:    %s\n", configPath)

	fmt.Println()

	// Final confirmation
	confirm := false
	confirmPrompt := &survey.Confirm{
		Message: "Save this configuration?",
		Default: true,
	}
	if err := survey.AskOne(confirmPrompt, &confirm); err != nil {
		return err
	}

	if !confirm {
		fmt.Println(yellow("Setup cancelled."))
		return nil
	}

	// Create and save configuration
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

	if err := backend.SaveConfig(config, configPath); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	// Success message
	fmt.Println()
	fmt.Println(greenBold("Configuration saved successfully!"))
	fmt.Println()

	// Next steps based on network
	fmt.Println(blueBold("Next Steps:"))
	fmt.Println(strings.Repeat("-", 15))

	if network == "testnet" {
		fmt.Println("1. Get free testnet tokens:")
		fmt.Println(blue("   Visit: https://faucet.sui.io/"))
		fmt.Println("   Select 'Testnet' and enter your address")
		fmt.Println()
	}

	fmt.Println("2. Upload your first file:")
	fmt.Println("   walrus-cli upload myfile.pdf")
	fmt.Println()

	fmt.Println("3. View your files:")
	fmt.Println("   walrus-cli list")
	fmt.Println()

	fmt.Println("4. Check status anytime:")
	fmt.Println("   walrus-cli status")
	fmt.Println()

	fmt.Println(magentaBold("Happy storing with Walrus!"))

	return nil
}

func getNetworkDisplay(network string) string {
	switch network {
	case "testnet":
		return green("Testnet")
	case "mainnet":
		return red("Mainnet")
	case "custom":
		return blue("Custom")
	default:
		return network
	}
}

// ModernStatusDisplay shows colorized status information
func ModernStatusDisplay(config *backend.Config) {
	fmt.Println()
	fmt.Println(cyanBold("Walrus CLI Status"))
	fmt.Println(strings.Repeat("=", 25))
	fmt.Println()

	// Network detection and display
	var network string
	if strings.Contains(config.Walrus.AggregatorURL, "testnet") {
		network = "testnet"
	} else if strings.Contains(config.Walrus.AggregatorURL, "walrus.space") {
		network = "mainnet"
	} else {
		network = "custom"
	}

	fmt.Println(blueBold("Network Configuration"))
	fmt.Printf("Network:        %s\n", getNetworkDisplay(network))
	fmt.Printf("Aggregator:     %s\n", config.Walrus.AggregatorURL)
	fmt.Printf("Publisher:      %s\n", config.Walrus.PublisherURL)
	fmt.Printf("Default Epochs: %d\n", config.Walrus.Epochs)

	// Wallet status
	fmt.Println()
	fmt.Println(yellowBold("Wallet Status"))
	if config.Walrus.Wallet.PrivateKey != "" {
		fmt.Printf("Wallet:         %s (%s...)\n",
			green("Configured"),
			config.Walrus.Wallet.PrivateKey[:15])
		fmt.Printf("Ready:          %s\n", green("Ready for uploads"))
	} else {
		fmt.Printf("Wallet:         %s\n", yellow("Not configured"))
		if network == "testnet" {
			fmt.Printf("Status:         %s\n", yellow("Can view/estimate, uploads may fail"))
		} else {
			fmt.Printf("Status:         %s\n", red("Wallet required for mainnet"))
		}
	}

	// Storage statistics
	fmt.Println()
	fmt.Println(greenBold("Storage Statistics"))
	index := loadIndex()

	var totalSize int64
	var validBlobs int
	for _, entry := range index.Files {
		totalSize += entry.Size
		if entry.BlobID != "" {
			validBlobs++
		}
	}

	fmt.Printf("Files Tracked:  %d\n", len(index.Files))
	fmt.Printf("Total Size:     %s\n", formatBytes(totalSize))
	fmt.Printf("Valid Blobs:    %s\n", green(fmt.Sprintf("%d/%d", validBlobs, len(index.Files))))

	if validBlobs > 0 {
		fmt.Println()
		fmt.Println(blueBold("Recent Uploads"))
		count := 0
		for name, entry := range index.Files {
			if entry.BlobID != "" && count < 3 {
				fmt.Printf("  %s %s (%s)\n",
					"•",
					cyan(name),
					formatBytes(entry.Size))
				count++
			}
		}
	}

	// Quick actions
	fmt.Println()
	fmt.Println(magentaBold("Quick Actions"))
	fmt.Printf("• %s\n", "walrus-cli setup    # Reconfigure")
	fmt.Printf("• %s\n", "walrus-cli upload   # Upload file")
	fmt.Printf("• %s\n", "walrus-cli list     # View files")
	fmt.Println()
}