
import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';

@Component({
    selector: 'convoy-empty-state, [convoy-empty-state]',
    imports: [],
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
