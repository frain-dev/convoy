import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';

@Component({
	selector: 'convoy-source-url',
	standalone: true,
	imports: [CommonModule, ButtonComponent, SelectComponent, CopyButtonComponent],
	templateUrl: './source-url.component.html'
})
export class SourceURLComponent implements OnInit {
	@Input('heading') heading!: string;
	@Input('url') url!: string;
	@Output('close') close = new EventEmitter<any>();

	constructor() {}

	ngOnInit(): void {}
}
