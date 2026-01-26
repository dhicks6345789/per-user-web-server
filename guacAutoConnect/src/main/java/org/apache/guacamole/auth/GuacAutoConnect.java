/**
 * A custom Guacamole authentication provider that automatically connects the given user to a remote desktop instance hosted inside a Docker container.
 * If a suitible instance matching the username doesn't already exist, one will be created.
 * This extension assumes something else is handling the actual authentication, and that by the time this code gets called the user is assumed to be valid and authorised.
 * Designed for use with Pangolin handling authentication and a VNC-enabled Desktop image available in Docker.
 * Also, designed to communicate (via a simple HTTP API) with a dedicated Session Manager serivce running on the Docker host. This component itself runs inside a container,
 * it can't directly create sibling containers, it has to pass the request up to its host for that.
 */
package org.apache.guacamole.auth;

// Standard libraries.
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
import java.net.http.HttpRequest.BodyPublishers;

// We're implementing Guacamole's SimpleAuthenticationProvider interface, we need to import some of Guacamole's libraries.
import org.apache.guacamole.GuacamoleException;
import org.apache.guacamole.net.auth.simple.SimpleAuthenticationProvider;
import org.apache.guacamole.net.auth.Credentials;
import org.apache.guacamole.protocol.GuacamoleConfiguration;

// For parsing JSON strings to objects.
import org.json.JSONObject;

// For handling logging.
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

// The main class, which Guacamole sees as an Authentication Provider extension.
public class GuacAutoConnect extends SimpleAuthenticationProvider {
  // Initialize the logger for this class.
  private static final Logger logger = LoggerFactory.getLogger(GuacAutoConnect.class);
  
  // Tell Guacamole what the name of this custom Guacamole extension is.
  @Override public String getIdentifier() {
    return "guac-auto-connect";
  }

  // This function gets called when a user succesfully logs in.
  @Override public Map<String, GuacamoleConfiguration> getAuthorizedConfigurations(Credentials credentials) throws GuacamoleException {
    // Create a new map of Guacamole configurations to return. If we can't find / create a desktop instance to connect to, this will stay empty and result in an error for the user.
    Map<String, GuacamoleConfiguration> guacConfigs = new HashMap<String, GuacamoleConfiguration>();
    
    // Figure out the username of the user who has just logged in.
    String username = credentials.getUsername().split("@")[0];
    
    // Output a log message. We simply write to STDOUT, where the output can be displayed by Docker.
    logger.info("User " + username + " connected to Guacamole - contacing Session Manager for session details.");

    // Call the Session Manager service (a basic, self-contained HTTP server written in Go) to tell it the user wants to connect to a VNC desktop instance.
    // We pass in the username, if there's a free slot available we should get back a VNC port number and password.
    HttpClient sessionManagerClient = HttpClient.newHttpClient();
    HttpRequest sessionManagerRequest = HttpRequest.newBuilder().uri(URI.create("http://host.docker.internal:8091/connectOrStartSession")).header("Content-Type", "application/x-www-form-urlencoded").POST(BodyPublishers.ofString("username=" + username)).build();
    try {
      HttpResponse<String> sessionManagerResponse = sessionManagerClient.send(sessionManagerRequest, HttpResponse.BodyHandlers.ofString());
      logger.info("Session Manager responded: " + sessionManagerResponseesponse.body());
      
      // Parse the JSON data returned from the Session Manager. To do: probably best to check for error messages first.
      JSONObject obj = new JSONObject(sessionManagerResponse.body());
      String desktopPort = obj.getString("portNumber");
      String VNCPassword = obj.getString("password");
      
      if (desktopPort.equals("")) {
        logger.info("Problem finding / starting desktop instance for user " + username);
      } else {
        logger.info("Connecting user " + username + " to desktop on port " + desktopPort);
      
        // Create a new configuration object to return to Guacamole. This will contain details for the one connection to the user's indidvidual remote desktop.
        GuacamoleConfiguration guacConfig = new GuacamoleConfiguration();
    
        // Set protocol and connection parameters.
        guacConfig.setProtocol("vnc");
        guacConfig.setParameter("hostname", "desktop-" + username);
        guacConfig.setParameter("port", desktopPort);
        guacConfig.setParameter("username", username);
        guacConfig.setParameter("password", VNCPassword);
        guacConfigs.put("Developer Desktop: " + username, guacConfig);
      }
    } catch (java.io.IOException | java.lang.InterruptedException e) {
      System.err.println("An error occurred while calling the Session Manager service: " + e.getMessage());
      e.printStackTrace();
    }
    return guacConfigs;
  }
}
