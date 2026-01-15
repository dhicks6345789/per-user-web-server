package org.apache.guacamole.auth;

import java.util.Map;
import java.util.HashMap;
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
  private static final Logger logger = LoggerFactory.getLogger(MyCustomAuthPlugin.class);
  
  @Override public String getIdentifier() {
    return "guac-auto-connect";
  }
  
  @Override public Map<String, GuacamoleConfiguration> getAuthorizedConfigurations(Credentials credentials) throws GuacamoleException {
    // Output a log message.
    logger.info(guac-auto-connect: User " + credentials.getUsername() +" logged in.");
    
    // Create a new configuration object to return to Guacamole. This will contain details for the one connection to the user's indidvidual remote desktop.
    Map<String, GuacamoleConfiguration> configs = new HashMap<String, GuacamoleConfiguration>();
    GuacamoleConfiguration config = new GuacamoleConfiguration();

    // Set protocol and connection parameters.
    config.setProtocol("vnc");
    config.setParameter("hostname", "desktop");
    config.setParameter("port", "5901");
    config.setParameter("username", "desktopuser");
    config.setParameter("password", "vncpassword");
    configs.put(credentials.getUsername() + ": Developer Desktop", config);
    return configs;
  }
}
