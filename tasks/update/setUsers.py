import csv

print("Setting users in Pangolin...", flush=True)

with open("users.csv", newline="\n") as usersFile:
    usersReader = csv.reader(usersFile, delimiter=",", quotechar="\"")
    for user in usersReader:
        print(", ".join(user), flush=True)
