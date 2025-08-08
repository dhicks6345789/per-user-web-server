# The Pandas data-processing library.
import pandas

# Our own set of handy functions.
import pangolinLib



print("Setting users in Pangolin...", flush=True)

users = pandas.read_csv("data/users.csv", header=0)
for userIndex, userRow in users.iterrows():
    print(userRow, flush=True)

config = pangolinLib.loadConfig(configFile="config.json", requiredParameters=["Pangolin", "PangolinAPIBaseURL"])
print(pangolinLib.APICall(config["Pangolin"]["APIKey"], config["PangolinAPIBaseURL"], ""))
