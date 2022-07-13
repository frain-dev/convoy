import { CommonModule } from '@angular/common';
import { Component, Input, OnInit } from '@angular/core';
import { ButtonComponent } from '../button/button.component';

@Component({
	selector: 'convoy-tooltip',
	standalone: true,
	imports: [CommonModule, ButtonComponent],
	templateUrl: './tooltip.component.html',
	styleUrls: ['./tooltip.component.scss']
})
export class TooltipComponent implements OnInit {
	@Input('size') size: 'sm'| 'md' = 'md';
	@Input('position') position: 'left'| 'right' = 'left';
  
	constructor() {}

	ngOnInit(): void {}
}
