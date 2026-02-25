#!/usr/bin/bash
cd /usr/local/bin/pinta-3.1.1
sudo chown -R $USER:$USER Pinta.Core/obj/Debug/net8.0
sudo chown -R $USER:$USER Pinta.Docking/obj/Debug/net8.0
sudo chown -R $USER:$USER Pinta.Effects/obj/Debug/net8.0
sudo chown -R $USER:$USER Pinta.Gui.Addins/obj/Debug/net8.0
sudo chown -R $USER:$USER Pinta.Gui.Widgets/obj/Debug/net8.0
sudo chown -R $USER:$USER Pinta.Tools/obj/Debug/net8.0
sudo chown -R $USER:$USER Pinta/obj/Debug
DOTNET_ROLL_FORWARD=Major dotnet run --project Pinta
