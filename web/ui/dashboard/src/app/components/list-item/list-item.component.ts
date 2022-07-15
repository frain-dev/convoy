import { Component, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
	selector: 'convoy-list-item',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './list-item.component.html',
	styleUrls: ['./list-item.component.scss']
})
export class ListItemComponent implements OnInit {
	@Input('className') class!: string;
	@Input('hover') hover: boolean = false;
	@Input('active') active: boolean = false;

	constructor() {}

	ngOnInit(): void {}
}
