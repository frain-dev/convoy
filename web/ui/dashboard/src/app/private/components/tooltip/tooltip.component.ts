import { Component, Input, OnInit } from '@angular/core';

@Component({
	selector: 'app-tooltip',
	templateUrl: './tooltip.component.html',
	styleUrls: ['./tooltip.component.scss']
})
export class TooltipComponent implements OnInit {
	@Input() size: string = 'normal';
	@Input() position: string = 'left';

	constructor() {}

	ngOnInit(): void {}
}
