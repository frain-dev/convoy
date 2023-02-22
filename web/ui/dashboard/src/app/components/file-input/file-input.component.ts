import { Component, EventEmitter, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
	selector: 'convoy-file-input',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './file-input.component.html',
	styleUrls: ['./file-input.component.scss']
})
export class FileInputComponent implements OnInit {
	file: any;
	@Output('selectedFile') selectedFile = new EventEmitter();
	@Output('deleteFile') deleteFile = new EventEmitter();
	constructor(private generalService: GeneralService) {}

	ngOnInit(): void {}

	parseFile(event: any) {
		const fileDetails = event.target.files[0];
		if (fileDetails.size > 5000 || fileDetails.type !== 'application/json') {
			this.generalService.showNotification({ message: 'Please uplaed a JSON file not larger than 5kb', style: 'warning' });
			return;
		}
		this.selectedFile.emit(fileDetails);
		this.file = fileDetails;
	}

	fileSize(size: number) {
		let fileSize;
		if (size < 1000000) fileSize = `${Math.ceil(size / 1000)} kb`;
		else fileSize = `${Math.ceil(size / 1000000)} mb`;

		return fileSize;
	}
}
