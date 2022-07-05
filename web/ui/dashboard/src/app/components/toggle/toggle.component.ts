import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';

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
	@Output('onChange') onChange = new EventEmitter<any>();

	ngOnInit(): void {}
}
