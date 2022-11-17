import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { InputComponent } from 'src/app/components/input/input.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { FormBuilder, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { CardComponent } from 'src/app/components/card/card.component';

@Component({
	selector: 'convoy-create-portal-link',
	standalone: true,
	imports: [CommonModule, ModalComponent, InputComponent, SelectComponent, CardComponent, ReactiveFormsModule],
	templateUrl: './create-portal-link.component.html',
	styleUrls: ['./create-portal-link.component.scss']
})
export class CreatePortalLinkComponent implements OnInit {
	portalLinkForm: FormGroup = this.formBuilder.group({
		linkName: [],
		endpoint: [],
		url: []
	});

	constructor(private formBuilder: FormBuilder) {}

	ngOnInit(): void {}

    savePortalLink(){

    }
}
