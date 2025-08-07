import os
import csv

print("Generating test data.", flush=True)

if not os.path.exists("data"):
    os.makedirs("data")

with open("data/users.csv", "w", newline="\n") as usersFile:
    usersWriter = csv.writer(usersFile, delimiter=",", quotechar="\"", quoting=csv.QUOTE_ALL)
    usersWriter.writerow(["User One", "user1"])
    usersWriter.writerow(["User Two", "user2"])
    usersWriter.writerow(["User Three", "user3"])
