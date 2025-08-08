import pandas

print("Setting users in Pangolin...", flush=True)

users = pandas.read_csv("data/users.csv", header=0)
for userIndex, userRow in users.iterrows():
    print(userRow, flush=True)
