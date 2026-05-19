package nodes

import "strings"

// LocalizeContext transforms product information to Nigerian context.
// This provides cultural adaptation for the review generation process.
func LocalizeContext(product *TargetProduct) TargetProduct {
	localized := *product

	// Localize source/marketplace names
	localized.Source = localizeSource(localized.Source)

	// Localize currency if needed
	localized.Currency = localizeCurrency(localized.Currency)

	// Add Nigerian-specific context to description
	if localized.Description != "" {
		localized.Description = addNigerianContext(localized.Description, localized.ProductCategory)
	}

	return localized
}

// localizeSource maps international marketplace names to Nigerian equivalents.
func localizeSource(source string) string {
	lowerSource := strings.ToLower(strings.TrimSpace(source))

	sourceMap := map[string]string{
		"amazon":      "Jumia",
		"amazon.com":  "Jumia",
		"ebay":        "Konga",
		"aliexpress":  "Jumia Global",
		"etsy":        "Local Artisans Platform",
		"goodreads":   "Goodreads",
		"yelp":        "Bookings.com/Google Maps",
		"uber eats":   "Bolt Food/Jumia Food",
		"grubhub":     "Jumia Food",
		"walmart":     "Shoprite",
		"target":      "Shoprite",
		"best buy":    "Slot.ng/Takealot",
		"apple store": "iSpire/Apple Authorized",
		"google play": "Google Play",
		"itunes":      "iTunes/Apple Music",
		"spotify":     "Spotify",
		"netflix":     "Netflix",
		"airbnb":      "Airbnb",
	}

	if mapped, exists := sourceMap[lowerSource]; exists {
		return mapped
	}

	return source
}

// localizeCurrency converts common currency codes to Nigerian Naira context.
func localizeCurrency(currency string) string {
	upperCurr := strings.ToUpper(strings.TrimSpace(currency))

	currencyMap := map[string]string{
		"USD": "NGN",
		"EUR": "NGN",
		"GBP": "NGN",
		"JPY": "NGN",
		"CNY": "NGN",
		"INR": "NGN",
	}

	if mapped, exists := currencyMap[upperCurr]; exists {
		// In a real implementation, you'd calculate exchange rates
		// For now, we just mark it as NGN
		return mapped
	}

	// If already NGN, keep it
	if upperCurr == "NGN" {
		return currency
	}

	// Default to NGN
	return "NGN"
}

// addNigerianContext enriches product descriptions with Nigerian relevance.
func addNigerianContext(description string, category string) string {
	contextAdditions := map[string]string{
		"electronics": " (Reliable for Nigeria's climate and power-sensitive environment)",
		"phone":       " (Check power consumption and local network compatibility)",
		"laptop":      " (Consider Nigeria's power situation; may need UPS)",
		"tablet":      " (Good for offline use given intermittent connectivity)",
		"power bank":  " (Essential in Nigeria's power-unstable environment)",
		"generator":   " (Crucial for Nigerian homes and offices)",
		"inverter":    " (Useful backup power source)",
		"appliance":   " (Check international warranty coverage for Nigeria)",
		"book":        " (Good for personal development and learning)",
		"cosmetics":   " (Verify ingredients compatibility with tropical climate)",
		"medicine":    " (Ensure storage conditions suit Nigeria's tropical heat)",
		"food":        " (Check if suitable for Nigerian climate and storage)",
		"beverage":    " (Verify cold chain management in Nigeria)",
		"fashion":     " (Consider Nigeria's tropical climate and fashion trends)",
		"shoes":       " (Durability important in Nigerian conditions)",
		"clothing":    " (Light fabrics better for Nigerian weather)",
	}

	lowerCat := strings.ToLower(category)
	for cat, addition := range contextAdditions {
		if strings.Contains(lowerCat, cat) {
			// Only add if not already present
			if !strings.Contains(description, addition) {
				return description + addition
			}
		}
	}

	return description
}

// GetNigerianContactInfo provides context-aware guidance for product interactions in Nigeria.
func GetNigerianContactInfo(productCategory string) string {
	contactInfo := map[string]string{
		"electronics": "For warranty support: Check if product has local Nigerian service centers",
		"appliance":   "Ensure appliance can handle Nigeria's variable power supply (150-240V fluctuations)",
		"medicine":    "Verify pharmacy recommendations for Nigerian climate storage (typically 25°C environment control)",
		"food":        "Check if item is refrigerated and review cold chain management for Nigerian delivery",
		"fashion":     "Consider if product suits Nigeria's tropical climate (high humidity, intense sun)",
	}

	lowerCat := strings.ToLower(productCategory)
	for cat, info := range contactInfo {
		if strings.Contains(lowerCat, cat) {
			return info
		}
	}

	return ""
}

// LocalizationRegistry holds localization rules for specific product types.
type LocalizationRegistry struct {
	// Can be expanded with more sophisticated localization rules
	Markets    map[string]string // Map of international to local markets
	Currencies map[string]string // Currency conversion preferences
	Categories map[string]string // Category-specific adaptations
}

// NewLocalizationRegistry creates a new registry with predefined Nigerian localizations.
func NewLocalizationRegistry() *LocalizationRegistry {
	return &LocalizationRegistry{
		Markets: map[string]string{
			"amazon":     "Jumia",
			"ebay":       "Konga",
			"aliexpress": "Jumia Global",
		},
		Currencies: map[string]string{
			"USD": "NGN",
			"EUR": "NGN",
		},
		Categories: map[string]string{
			"electronics":    "electronics",
			"power_tools":    "power_tools",
			"home_appliance": "appliance",
		},
	}
}
