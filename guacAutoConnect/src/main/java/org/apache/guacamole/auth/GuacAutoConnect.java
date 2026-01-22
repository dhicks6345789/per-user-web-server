package org.apache.guacamole.auth;

import java.util.Map;
import java.util.HashMap;
import java.util.List;
import java.util.ArrayList;

import java.io.IOException;
import java.io.BufferedReader;
import java.io.InputStreamReader;

import java.lang.InterruptedException;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;

import org.apache.guacamole.GuacamoleException;
import org.apache.guacamole.net.auth.simple.SimpleAuthenticationProvider;
import org.apache.guacamole.net.auth.Credentials;
import org.apache.guacamole.protocol.GuacamoleConfiguration;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

// Temporary?
import java.util.stream.Collectors;
import java.net.http.HttpRequest.BodyPublishers;

/**
 * A custom authentication provider that automatically connects the given user to a remote desktop instance hosted inside a Docker container.
 * If a suitible instance matching the username doesn't already exist, one will be created.
 * This extension assumes something else is handling the actual authentication, and that by the time this code gets called the user is assumed to be valid and authorised.
 * Designed for use with Pangolin handling authentication and a VNC-enabled Desktop image available in Docker.
 */
public class GuacAutoConnect extends SimpleAuthenticationProvider {
  // Initialize the logger for this class.
  private static final Logger logger = LoggerFactory.getLogger(GuacAutoConnect.class);

  // A helper function that runs an external command, returning all the output produced as an array of Strings.
  private static String[] runCommand(String... parameters) {
    List<String> output = new ArrayList<>();
    
    // Log a message to the console.
    logger.info("Running command: " + String.join(" ", parameters));
    
    try {
      // Initialize ProcessBuilder with the command and its arguments.
      ProcessBuilder processBuilder = new ProcessBuilder(parameters);
    
      // Redirect the command's error stream to standard output so we capture everything.
      processBuilder.redirectErrorStream(true);

      Process process = processBuilder.start();
    
      // Use try-with-resources to ensure the reader closes automatically
      try (BufferedReader reader = new BufferedReader(new InputStreamReader(process.getInputStream()))) {
        String line;
        while ((line = reader.readLine()) != null) {
          output.add(line);
        }
      }
    
      // Wait for the process to finish and check the exit code
      int exitCode = process.waitFor();
      if (exitCode != 0) {
        return new String[] {"ERROR: " + exitCode};
      }
    } catch (Exception e) {
      e.printStackTrace();
    }
    return output.toArray(new String[0]);
  }
  
  // Tell Guacamole what the name of this custom extension is.
  @Override public String getIdentifier() {
    return "guac-auto-connect";
  }

  // This function gets called when a user succesfully logs in.
  @Override public Map<String, GuacamoleConfiguration> getAuthorizedConfigurations(Credentials credentials) throws GuacamoleException {
    // Create a new map of Guacamole configurations to return. If we can't find / create a desktop instance to connect to, this will stay empty and result in an error for the user.
    Map<String, GuacamoleConfiguration> configs = new HashMap<String, GuacamoleConfiguration>();
    
    // Figure out the username of the user who has just logged in.
    String username = credentials.getUsername().split("@")[0];
    
    // Output a log message. We simply write to STDOUT, where the output can be displayed by Docker.
    logger.info("User " + username + " logged in.");
    
    Map<String, String> sessionParameters = new HashMap<>();
    sessionParameters.put("username", username);
    // 2. Convert Map to form-url-encoded string
    String form = sessionParameters.entrySet().stream().map(e -> e.getKey() + "=" + e.getValue()).collect(Collectors.joining("&"));
    logger.info("form: " + form);
    HttpClient client = HttpClient.newHttpClient();
    //HttpRequest request = HttpRequest.newBuilder().uri(URI.create("http://host.docker.internal:8091/connectOrStartSession")).header("Content-Type", "application/x-www-form-urlencoded").POST(BodyPublishers.ofString(form)).build();
    HttpRequest request = HttpRequest.newBuilder().uri(URI.create("http://host.docker.internal:8091/connectOrStartSession")).header("Content-Type", "application/x-www-form-urlencoded").POST(BodyPublishers.ofString("{\"username\": \"" + username + "\"}")).build();
    
    try {
      HttpResponse<String> response = client.send(request, HttpResponse.BodyHandlers.ofString());
      logger.info("Status Code: " + response.statusCode());
      logger.info("Response: " + response.body());
      String desktopPort = "5901";
    
      /*
      // To do: unmount or re-use any existing user mount, make sure we don't double-up.
      // Mount the user's Google Drive home to /mnt in the container host, ready to be passed to the user's desktop container.
      String[] rcloneMountResult = runCommand("rclone", "mount", "gdrive:", "/mnt/" + username, "--allow-other", "--vfs-cache-mode", "writes", "--drive-impersonate", username + "@knightsbridgeschool.com", "&");
      logger.info(String.join("\n", rcloneMountResult));
    
      logger.info("Starting a new desktop instance for user " + username + " on port " + desktopPort);
      String[] dockerRunResult = runCommand("sudo", "docker", "run", "--detach", "--name", "desktop-" + username, "--expose", desktopPort, "--network", "pangolin_main", "sansay.co.uk-dockerdesktop:0.1-beta.3", "bash", "/home/desktopuser/startup.sh", "bananas", String.valueOf(vncDisplay));
      logger.info(String.join("\n", dockerRunResult));
    
      // Wait for the desktop instance to start up before we do anything more.
      // To do: maybe actually run docker ps -a more rather than just do a simple pause.
      try {
        Thread.sleep(2000);
      } catch (InterruptedException e) {
        Thread.currentThread().interrupt();
        // Gemini: Handle the exception (e.g., logging).
      }
      */
      
      if (desktopPort.equals("")) {
        logger.info("Problem finding / starting desktop instance for user " + username);
      } else {
        logger.info("Connecting user " + username + " to desktop on port " + desktopPort);
      
        // Create a new configuration object to return to Guacamole. This will contain details for the one connection to the user's indidvidual remote desktop.
        GuacamoleConfiguration config = new GuacamoleConfiguration();
    
        // Set protocol and connection parameters.
        config.setProtocol("vnc");
        config.setParameter("hostname", "desktop-" + username);
        config.setParameter("port", desktopPort);
        config.setParameter("username", "desktopuser");
        config.setParameter("password", "vncpassword");
        configs.put("Developer Desktop: " + username, config);
      }
    } catch (java.io.IOException | java.lang.InterruptedException e) {
      System.err.println("An error occurred while calling the Session Manager service: " + e.getMessage());
      e.printStackTrace();
    }
    return configs;
  }
}
