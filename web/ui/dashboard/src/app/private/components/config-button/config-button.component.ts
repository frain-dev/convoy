
import { Component, EventEmitter, OnInit, Output } from '@angular/core';

@Component({
    selector: 'convoy-config-button, [convoy-config-button]',
    templateUrl: './config-button.component.html',
    imports: []
})
export class ConfigButtonComponent implements OnInit {
	// @Input() isTransparent: boolean = false;
	// @Input() position: 'absolute' | 'fixed' | 'relative' = 'absolute';
	@Output('onClick') onClick = new EventEmitter<void>();

	constructor() {}

	ngOnInit(): void {}
}
