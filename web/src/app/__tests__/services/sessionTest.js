/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import * as utils from 'app/services/utils';
import * as apiResponse from 'app/__tests__/apiData';
import history from 'app/services/history';
import session, { BearerToken } from 'app/services/session';
import { LocalStorageKeys } from 'app/services/enums';
import { cfg, expect, $, api, spyOn } from 'app/__tests__/';

describe('services/session', () => {
    
  beforeEach(() => {        
    spyOn(session, '_startSessionChecker');
    spyOn(session, '_stopSessionChecker');    
    spyOn(history, 'push');    
    spyOn(localStorage, 'setItem');
    spyOn(localStorage, 'getItem');
    spyOn(localStorage, 'removeItem');
    spyOn(localStorage, 'clear');
  });

  afterEach(() => {
    expect.restoreSpies();
  })
  
  describe('logout()', () => {
    it('should clear localStorage, stop session checker, and redirect to login page', () => {      
      spyOn(api, 'delete').andReturn($.Deferred().resolve());      
      session.logout();      
      expect(api.delete).toHaveBeenCalledWith(cfg.api.sessionPath);  
      expect(session._stopSessionChecker).toHaveBeenCalled();      
      expect(localStorage.clear).toHaveBeenCalled();	    
      expect(history.push).toHaveBeenCalledWith(cfg.routes.login, true);  
    });
  });

  describe('ensureSession()', () => {
    const json = apiResponse.bearerToken;      
    const expiredBearerToken = new BearerToken(json);
    expiredBearerToken.created = 0;
        
    it('should renew token', () => {
      spyOn(utils, 'getBearerToken').andReturn(expiredBearerToken);      
      spyOn(utils, 'setBearerToken');      
      spyOn(api, 'post').andReturn($.Deferred().resolve(json));      
      session.ensureSession();
      expect(api.post).toHaveBeenCalledWith(cfg.api.renewTokenPath);      
      expect(localStorage.setItem).toHaveBeenCalledWith(LocalStorageKeys.TOKEN_RENEW, true);
      expect(localStorage.removeItem).toHaveBeenCalledWith(LocalStorageKeys.TOKEN_RENEW);      
      expect(session._startSessionChecker).toHaveBeenCalled();

      const newToken = utils.setBearerToken.getLastCall().arguments[0];
      expect(newToken.accessToken).toBe(json.token);
      expect(newToken.created).toBeGreaterThan(0);
    });

    it('should reject if token is invalid', () => {
      let wasRejected = false;
      spyOn(utils, 'getBearerToken').andReturn(null);
      session.ensureSession().fail(()=> wasRejected = true)
      expect(wasRejected).toBe(true);
    });
  });
})
