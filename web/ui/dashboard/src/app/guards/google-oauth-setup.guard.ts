import {Injectable} from '@angular/core';
import { Router } from '@angular/router';

@Injectable({
  providedIn: 'root'
})
export class GoogleOAuthSetupGuard  {
  constructor(private router: Router) {}

  canActivate(): boolean {
    const hasIdToken = localStorage.getItem('GOOGLE_OAUTH_ID_TOKEN');
    const hasUserInfo = localStorage.getItem('GOOGLE_OAUTH_USER_INFO');

    if (hasIdToken && hasUserInfo) {
      return true;
    }

    this.router.navigateByUrl('/login');
    return false;
  }
}
