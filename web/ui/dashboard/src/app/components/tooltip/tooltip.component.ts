import { CommonModule } from '@angular/common';
import { Component, Input, OnInit } from '@angular/core';

@Component({
	selector: 'convoy-tooltip',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './tooltip.component.html',
	styleUrls: ['./tooltip.component.scss']
})
export class TooltipComponent implements OnInit {
	@Input('size') size = 'normal';
	@Input('position') position = 'left';
  
	constructor() {}

	ngOnInit(): void {}
}
