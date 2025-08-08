# Reads the given file, returns the entire contents as a single string.
def readFile(theFilename):
	inHandle = open(theFilename)
	result = inHandle.read()
	inHandle.close()
	return result

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
def APICall(theAPIKey, theAPIBaseURL, theAPIURL):
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
				print("ERROR: Pangolin API return code: " + str(APIResponse.status_code), flush=True)
				exit()
			return APIResponse.json()
		retries = retries + 1

# A function that calls the Pangolin API, expecting a paged result.
def pagedAPICall(theAPIKey, theAPIBaseURL, theAPIURL):
	result = []
	pangolinData = {}
	pangolinData["next"] = theAPIURL
	while pangolinData["next"] != None:
		#pageNumber = urllib.parse.parse_qs(urllib.parse.urlparse(pangolinData["next"]).query)["page"][0]
		#print("Getting records - page " + pageNumber + "...", flush=True)
		pangolinData = pangolinAPICall(theAPIKey, theAPIBaseURL, pangolinData["next"])
		for pangolinItem in pangolinData["results"]:
			result.append(pangolinItem)
	return result
