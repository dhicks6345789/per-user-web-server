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
    // Output a log message. We simply write to STDOUT, where the output can be displayed by Docker.
    String username = credentials.getUsername().split("@")[0];
    logger.info("User " + username + " logged in.");
    
    // We want to get a list of running containers using our "dockerdesktop" image so we can see if there's one already running we can connect the user to or if we need to start a new one
    List<String[]> containerList = new ArrayList<>();
    
    // Call the Docker command to list all running docker desktop instances, see if any match the current user.
    ProcessBuilder processBuilder = new ProcessBuilder("docker", "ps", "-a");
    
    try {
      Process process = processBuilder.start();
      BufferedReader reader = new BufferedReader(new InputStreamReader(process.getInputStream()));
      
      String line;
      boolean isHeader = true;
      
      while ((line = reader.readLine()) != null) {
        if (isHeader) {
          isHeader = false; // Skip the column headers.
          continue;
        }
        
        // Regex split: looks for 2 or more consecutive spaces
        // This handles spaces within names or dates (e.g., "About an hour ago")
        String[] details = line.split("\\s{2,}");
        logger.info(details);
        containerList.add(details);
      }
      
      int exitCode = process.waitFor();
      if (exitCode != 0) {
        logger.info("Error: Docker command exited with code " + exitCode);
      }
    } catch (Exception e) {
      e.printStackTrace();
    }
    
    // Create a new configuration object to return to Guacamole. This will contain details for the one connection to the user's indidvidual remote desktop.
    Map<String, GuacamoleConfiguration> configs = new HashMap<String, GuacamoleConfiguration>();
    GuacamoleConfiguration config = new GuacamoleConfiguration();

    // Set protocol and connection parameters.
    config.setProtocol("vnc");
    config.setParameter("hostname", "desktop");
    config.setParameter("port", "5901");
    config.setParameter("username", "desktopuser");
    config.setParameter("password", "vncpassword");
    configs.put("Developer Desktop: " + username, config);
    return configs;
  }
}
