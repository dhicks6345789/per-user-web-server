package org.apache.guacamole.auth;

import java.util.Map;
import java.util.HashMap;
import java.util.List;
import java.util.ArrayList;
import java.io.BufferedReader;
import java.io.InputStreamReader;
import org.apache.guacamole.GuacamoleException;
import org.apache.guacamole.net.auth.simple.SimpleAuthenticationProvider;
import org.apache.guacamole.net.auth.Credentials;
import org.apache.guacamole.protocol.GuacamoleConfiguration;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * A custom authentication provider that automatically connects the given user to a remote desktop instance hosted inside a Docker container.
 * If a suitible instance matching the username doesn't already exist, one will be created.
 * This extension assumes something else is handling the actual authentication, and that by the time this code gets called the user is assumed to be valid and authorised.
 * Designed for use with Pangolin handling authentication and a VNC-enabled Desktop image available in Docker.
 */
public class GuacAutoConnect extends SimpleAuthenticationProvider {
  // Initialize the logger for this class.
  private static final Logger logger = LoggerFactory.getLogger(GuacAutoConnect.class);

  // Tell Guacamole what the name of this custom extension is.
  @Override public String getIdentifier() {
    return "guac-auto-connect";
  }

  // This function gets called when a user succesfully logs in.
  @Override public Map<String, GuacamoleConfiguration> getAuthorizedConfigurations(Credentials credentials) throws GuacamoleException {
    String processLine;
    int dockerPsProcessExitCode = 1;
    int dockerRunProcessExitCode = 1;
    String desktopPort = "";
    List<String> desktopPorts = new ArrayList<>();
    int vncDisplay = 0;
    ProcessBuilder processBuilder;
    
    // Output a log message. We simply write to STDOUT, where the output can be displayed by Docker.
    String username = credentials.getUsername().split("@")[0];
    logger.info("User " + username + " logged in.");
    
    // Call the Docker command to list all running docker desktop instances, see if any match the current user.
    processBuilder = new ProcessBuilder("sudo", "docker", "ps", "-a");
    
    try {
      Process process = processBuilder.start();
      BufferedReader reader = new BufferedReader(new InputStreamReader(process.getInputStream()));
      // Parse each line of the "docker ps -a" command to find all containers using our current "desktop" image, and record the port numbers used by each image.
      while ((processLine = reader.readLine()) != null) {
        // Regex split: looks for 2 or more consecutive spaces - this handles spaces within names or dates (e.g., "About an hour ago").
        String[] details = processLine.split("\\s{2,}");
        if (details[1].startsWith("sansay.co.uk-dockerdesktop")) {
          desktopPorts.add(details[5].split("/")[0]);
          // If we find a "desktop" image belonging to the current user, set that as the port number to connect to.
          if (details[6].startsWith("desktop-" + username)) {
            desktopPort = details[5].split("/")[0];
            vncDisplay = Integer.parseInt(desktopPort) - 5900;
          }
        }
      }
      dockerPsProcessExitCode = process.waitFor();
    } catch (Exception e) {
      e.printStackTrace();
    }
    
    if (dockerPsProcessExitCode == 0) {
      if (desktopPort.equals("")) {
        // If we don't already have a running "desktop" container associated with the current user, start one. First, we need to pick an available port number.
        for (vncDisplay = 5901; desktopPorts.contains(String.valueOf(vncDisplay)) && vncDisplay <= 5920; vncDisplay = vncDisplay + 1) {
        }
        desktopPort = String.valueOf(vncDisplay);
        vncDisplay = vncDisplay - 5900;
        // If we've run out of available ports, don't start a new instance.
        if (vncDisplay <= 20) {
          processBuilder = new ProcessBuilder("docker", "run", "--name", "desktop-" + username, "sansay.co.uk-dockerdesktop:0.1-beta.3", "vncserver", "-fg", "-localhost", "no", "-geometry", "1280x720", ":" + String.valueOf(vncDisplay), "&");
          try {
            Process process = processBuilder.start();
            BufferedReader reader = new BufferedReader(new InputStreamReader(process.getInputStream()));
            dockerRunProcessExitCode = process.waitFor();
          } catch (Exception e) {
            e.printStackTrace();
          }
        } else {
          logger.info("Desktop instances limit reached, unable to start new desktop instance for user " + username);
        }
      }
    } else {
      logger.info("Error: Docker ps command exited with code " + dockerPsProcessExitCode);
    }

    // Create a new map of Guacamole configurations to return. If we couldn't find / create a desktop instance to connect to, this will stay empty and result in an error for the user.
    Map<String, GuacamoleConfiguration> configs = new HashMap<String, GuacamoleConfiguration>();
    
    if (desktopPort.equals("")) {
      logger.info("Problem finding / starting desktop instance for user " + username);
    } else {
      logger.info("Connecting user " + username + " to desktop on port " + desktopPort);
      
      // Create a new configuration object to return to Guacamole. This will contain details for the one connection to the user's indidvidual remote desktop.
      GuacamoleConfiguration config = new GuacamoleConfiguration();
    
      // Set protocol and connection parameters.
      config.setProtocol("vnc");
      config.setParameter("hostname", "desktop");
      config.setParameter("port", "5901");
      config.setParameter("username", "desktopuser");
      config.setParameter("password", "vncpassword");
      configs.put("Developer Desktop: " + username, config);
    }
    return configs;
  }
}
