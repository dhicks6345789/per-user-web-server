package org.apache.guacamole.auth;

import java.util.Map;
import org.apache.guacamole.GuacamoleException;
import org.apache.guacamole.net.auth.simple.SimpleAuthenticationProvider;
import org.apache.guacamole.net.auth.Credentials;
import org.apache.guacamole.protocol.GuacamoleConfiguration;

/**
 * Authentication provider implementation intended to demonstrate basic use
 * of Guacamole's extension API. The credentials and connection information for
 * a single user are stored directly in guacamole.properties.
 */
public class TutorialAuthenticationProvider extends SimpleAuthenticationProvider {
  @Override
  public String getIdentifier() {
    return "guac-auto-connect";
  }
  
  @Override
  public Map<String, GuacamoleConfiguration>
  getAuthorizedConfigurations(Credentials credentials)
  throws GuacamoleException {
    // Do nothing ... yet
    return null;
  }
}
