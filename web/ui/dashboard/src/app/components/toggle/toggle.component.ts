import { CommonModule } from '@angular/common';
import { Component, Input, OnInit } from '@angular/core';

@Component({
	selector: 'convoy-toggle',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './toggle.component.html',
	styleUrls: ['./toggle.component.scss']
})
export class ToggleComponent implements OnInit {
	constructor() {}
	@Input('isChecked') isChecked = false;
	@Input('label') label!: string;
	ngOnInit(): void {}
}
