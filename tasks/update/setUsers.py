# Standard libraries.
import time
import json
import requests

# The Pandas data-processing library.
import pandas



# Takes an array of strings, then checks the JSON config file to make sure the required parameters are indeed set.
# Returns an array of configuration values.
def loadConfig(configFile="config.json", requiredParameters=[]):
	# Load the configuration file.
	config = json.loads(readFile(configFile))
	for requiredParameter in requiredParameters:
		if not requiredParameter in config.keys():
			print("Error - required value " + requiredParameter + " not set in " + configFile + ".", flush=True)
			sys.exit(1)
	return config
    
# A function to call the Pangolin API. Handles "Retry-After" errors by pausing
# and retrying after the defined delay, but simply exits on all other errors.
def pangolinAPICall(theAPIKey, theAPIBaseURL, theAPIURL):
	# A small delay so we don't hit the API rate limit.
	time.sleep(1)
	
	APIURL = theAPIURL
	if not APIURL.startswith(theAPIBaseURL):
		APIURL = theAPIBaseURL + APIURL

	retries = 0
	while retries < 2:
		APIResponse = requests.get(APIURL, headers = {"Authorization": "token " + theAPIKey})
		if APIResponse.status_code == 429:
			retrySeconds = int(APIResponse.headers["Retry-After"])
			print("WARNING: Pangolin API rate limit hit - pausing for " + str(retrySeconds) + " seconds.", flush=True)
			time.sleep(retrySeconds);
		else:
			if APIResponse.status_code != 200:
				print("ERROR: Tutorcruncher API return code: " + str(APIResponse.status_code), flush=True)
				exit()
			return APIResponse.json()
		retries = retries + 1



print("Setting users in Pangolin...", flush=True)

users = pandas.read_csv("data/users.csv", header=0)
for userIndex, userRow in users.iterrows():
    print(userRow, flush=True)

config = loadConfig(configFile="config.json", requiredParameters=["Pangolin", "PangolinAPIBaseURL"])
orgs = dukesLib.tutorcruncherPagedAPICall(config["Pangolin"]["APIKey"], config["PangolinAPIBaseURL"], "orgs")
