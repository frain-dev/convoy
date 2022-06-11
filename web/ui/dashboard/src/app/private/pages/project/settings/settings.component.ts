import { Component, OnInit } from '@angular/core';
import { PrivateService } from 'src/app/private/private.service';

@Component({
	selector: 'app-settings',
	templateUrl: './settings.component.html',
	styleUrls: ['./settings.component.scss']
})
export class SettingsComponent implements OnInit {
	constructor(public privateService: PrivateService) {}

	ngOnInit(): void {}
}
