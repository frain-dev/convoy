import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';

import { ButtonComponent } from 'src/app/components/button/button.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';

@Component({
    selector: 'convoy-source-url',
    imports: [ButtonComponent, CopyButtonComponent],
    templateUrl: './source-url.component.html'
})
export class SourceURLComponent implements OnInit {
	@Input('heading') heading!: string;
	@Input('url') url!: string;
	@Output('close') close = new EventEmitter<any>();

	constructor() {}

	ngOnInit(): void {}
}
