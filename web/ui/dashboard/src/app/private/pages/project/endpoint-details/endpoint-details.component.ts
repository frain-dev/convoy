import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';

@Component({
	selector: 'convoy-endpoint-details',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './endpoint-details.component.html',
	styleUrls: ['./endpoint-details.component.scss']
})
export class EndpointDetailsComponent implements OnInit {
	constructor(public privateService: PrivateService) {}

	ngOnInit(): void {}
}
