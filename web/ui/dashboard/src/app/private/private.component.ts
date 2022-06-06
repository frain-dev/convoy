import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { GeneralService } from '../services/general/general.service';

@Component({
  selector: 'app-private',
  templateUrl: './private.component.html',
  styleUrls: ['./private.component.scss']
})
export class PrivateComponent implements OnInit {
	showDropdown = false;
	apiURL = this.generalService.apiURL();

  constructor(private generalService: GeneralService, private router: Router) { }

  ngOnInit(): void {
  }

  logout() {
      localStorage.removeItem('CONVOY_AUTH');
      this.router.navigateByUrl('/login');
  }

  authDetails() {
      const authDetails = localStorage.getItem('CONVOY_AUTH');
      return authDetails ? JSON.parse(authDetails) : false;
  }

  requestToken(): string {
      if (this.authDetails()) {
          const { username, password } = this.authDetails();
          return btoa(`${username + ':' + password}`);
      } else {
          return '';
      }
  }

}
