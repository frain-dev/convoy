import { Component, EventEmitter, OnInit, Output, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CardComponent } from 'src/app/components/card/card.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { MonacoComponent } from '../monaco/monaco.component';
import { DialogHeaderComponent } from 'src/app/components/dialog/dialog.directive';
import { CreateProjectComponentService } from '../create-project-component/create-project-component.service';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
	selector: 'convoy-upload-events',
	standalone: true,
	imports: [CommonModule, CardComponent, ButtonComponent, MonacoComponent, DialogHeaderComponent],
	templateUrl: './upload-events.component.html',
	styleUrls: ['./upload-events.component.scss']
})
export class UploadEventsComponent implements OnInit {
	@Output('close') close: EventEmitter<any> = new EventEmitter();
	@ViewChild('requestEditor') requestEditor!: MonacoComponent;
	events: any;

	constructor(private createProjectService: CreateProjectComponentService, private generalService: GeneralService) {}

	ngOnInit(): void {}

	async addOpenApiSpec() {
		const open_api_spec = btoa(this.requestEditor?.getValue());

		try {
			const response = await this.createProjectService.addOpenApiSpecToCatalogue({ open_api_spec });
			this.generalService.showNotification({ message: response.message, style: 'success' });
            this.close.emit('apiSpecAdded')
			console.log(response);
		} catch {}
	}
}
