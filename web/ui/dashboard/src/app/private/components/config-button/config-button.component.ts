import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { ButtonComponent } from 'src/app/components/button/button.component';

@Component({
	selector: 'convoy-config-button, [convoy-config-button]',
	templateUrl: './config-button.component.html',
	standalone: true,
	imports: [CommonModule, ButtonComponent]
})
export class ConfigButtonComponent implements OnInit {
	// @Input() isTransparent: boolean = false;
	// @Input() position: 'absolute' | 'fixed' | 'relative' = 'absolute';
	@Output('onClick') onClick = new EventEmitter<void>();

	constructor() {}

	ngOnInit(): void {}
}
