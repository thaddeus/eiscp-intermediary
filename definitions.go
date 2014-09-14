package main

// Used to define a property, partly to figure out what data type it is
// and partly to help provide verbosity/human readability
type propertyDefinition struct {
	name           string // Proper name of the property
	description    string // Description of what this changes
	stringResponse bool   // Is this a non-hex value when returned?
	valueDefined   bool   // Does this property have special value definitions?
}

// Used for things like Input selectors which reference
// values to names like "STB/DVR". This is a verbosity object.
type valueDefinition struct {
	description string // The special definition
}

// Used for mapping definitions to values of commands
type valueDefKey struct {
	property propertyDefinition // The propertyDefinition we're getting a value for
	value    string             // The value of the property we want a definition for
}

// This will need to be expanded. I'll flush it out with my needs, but add your own for advanced properties if you wish.
// Feel free to pull request them if you do, I'm sure you'll make someone else's day.
var propertiesDictionary map[string]propertyDefinition = map[string]propertyDefinition{
	"PWR": propertyDefinition{
		name:           "Power",
		description:    "Device power or standby status",
		stringResponse: false,
		valueDefined:   false,
	},
	"AMT": propertyDefinition{
		name:           "Audio Muting",
		description:    "Master device mute",
		stringResponse: false,
		valueDefined:   false,
	},
	"MVL": propertyDefinition{
		name:           "Master Volume",
		description:    "The primary/master zone volume",
		stringResponse: false,
		valueDefined:   false,
	},
	"SLI": propertyDefinition{
		name:           "Input Selection",
		description:    "The primary/master input selection",
		stringResponse: false,
		valueDefined:   true,
	},
}

// This needs to be expanded along with the propertiesDictionary.
var valueDictionary map[valueDefKey]valueDefinition = map[valueDefKey]valueDefinition{
	// SLI values
	valueDefKey{propertiesDictionary["SLI"], "00"}: valueDefinition{
		description: "VIDEO1, VCR/DVR, STB/DVR",
	},
	valueDefKey{propertiesDictionary["SLI"], "01"}: valueDefinition{
		description: "VIDEO2, CBL/SAT",
	},
	valueDefKey{propertiesDictionary["SLI"], "02"}: valueDefinition{
		description: "VIDEO3, GAME/TV, GAME, GAME1",
	},
	valueDefKey{propertiesDictionary["SLI"], "03"}: valueDefinition{
		description: "VIDEO4, AUX1(AUX)",
	},
	valueDefKey{propertiesDictionary["SLI"], "04"}: valueDefinition{
		description: "VIDEO5, AUX2, GAME2",
	},
	valueDefKey{propertiesDictionary["SLI"], "05"}: valueDefinition{
		description: "VIDEO6, PC",
	},
	valueDefKey{propertiesDictionary["SLI"], "10"}: valueDefinition{
		description: "DVD, BD/DVD",
	},
	valueDefKey{propertiesDictionary["SLI"], "29"}: valueDefinition{
		description: "USB/USB(Front)",
	},
	valueDefKey{propertiesDictionary["SLI"], "2A"}: valueDefinition{
		description: "USB(Rear)",
	},
	valueDefKey{propertiesDictionary["SLI"], "2B"}: valueDefinition{
		description: "NETWORK, NET",
	},
	valueDefKey{propertiesDictionary["SLI"], "2C"}: valueDefinition{
		description: "USB(Toggle)",
	},
}
