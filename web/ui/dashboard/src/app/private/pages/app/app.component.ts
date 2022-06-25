import { Component, OnInit } from '@angular/core';
import { DomSanitizer } from '@angular/platform-browser';
import { ActivatedRoute } from '@angular/router';

@Component({
	selector: 'app-app',
	templateUrl: './app.component.html',
	styleUrls: ['./app.component.scss']
})
export class AppComponent implements OnInit {
	token: string = this.route.snapshot.params.token;
	iframeURL = this.sanitizer.bypassSecurityTrustResourceUrl(`/app/${this.token}`);

	constructor(private route: ActivatedRoute, private sanitizer: DomSanitizer) {}

	ngOnInit() {}
}
