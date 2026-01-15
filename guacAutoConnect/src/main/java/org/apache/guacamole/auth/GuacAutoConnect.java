package org.apache.guacamole.auth;

//import java.util.Map;
import java.util.HashMap;
import org.apache.guacamole.GuacamoleException;
import org.apache.guacamole.net.auth.simple.SimpleAuthenticationProvider;
import org.apache.guacamole.net.auth.Credentials;
import org.apache.guacamole.protocol.GuacamoleConfiguration;

/**
 * Authentication provider implementation intended to demonstrate basic use
 * of Guacamole's extension API. The credentials and connection information for
 * a single user are stored directly in guacamole.properties.
 */
public class GuacAutoConnect extends SimpleAuthenticationProvider {
  @Override
  public String getIdentifier() {
    return "guac-auto-connect";
  }
  
  @Override
  public Map<String, GuacamoleConfiguration> getAuthorizedConfigurations(Credentials credentials) throws GuacamoleException {
    // Do nothing ... yet
    System.out.println("guac-auto-connect: User " + credentials.getUsername() +" logged in.");
    
    // Successful login. Return configurations.
    Map<String, GuacamoleConfiguration> configs = new HashMap<String, GuacamoleConfiguration>();
    
    // Create new configuration.
    GuacamoleConfiguration config = new GuacamoleConfiguration();

    // Set protocol and connection parameters.
    config.setProtocol("VNC");
    config.setParameter("hostname", "desktop");
    config.setParameter("port", "5901");
    config.setParameter("username", "desktopuser");
    config.setParameter("password", "vncpassword");
    configs.put("Desktop Connection", config);
    return configs;
  }
}
