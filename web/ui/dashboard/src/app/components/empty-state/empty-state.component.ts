import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { ButtonComponent } from '../button/button.component';

@Component({
	selector: 'convoy-empty-state, [convoy-empty-state]',
	standalone: true,
	imports: [CommonModule, ButtonComponent],
	templateUrl: './empty-state.component.html'
})
export class EmptyStateComponent implements OnInit {
	@Input('imgSrc') imgSrc!: string;
	@Input('heading') heading!: string;
	@Input('description') description!: string;
	@Input('buttonText') buttonText!: string;
	@Output('onAction') onAction = new EventEmitter<any>();

	constructor() {}

	ngOnInit(): void {}
}
