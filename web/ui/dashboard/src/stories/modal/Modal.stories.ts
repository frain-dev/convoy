import { Story, Meta } from '@storybook/angular/types-6-0';
import { ModalComponent } from '../../app/components/modal/modal.component';

export default {
	title: 'Example/Modal',
	component: ModalComponent,
	argTypes: {
		position: {
			options: ['full', 'left', 'right', 'center'],
			control: { type: 'select' },
			defaultValue: 'right'
		},
		size: {
			options: ['sm', 'md', 'lg'],
			control: { type: 'select' },
			defaultValue: 'md'
		},
		title: {
			control: { type: 'text' }
		}
	}
} as Meta;

const Template: Story<ModalComponent> = (args: ModalComponent) => ({
	props: args,
	template: `<convoy-modal [position]="position" [size]="size" [title]="title">
                <div class="pb-200px" modalBody>{{ngContent}}</div>
                <div modalFooter>convoy modal footer</div>
               </convoy-modal>`
});

export const Base = Template.bind({});
Base.args = {
	ngContent: 'convoy modal content',
	position: 'right',
    size: 'md',
    title: 'Convoy modal title'
} as Partial<ModalComponent>;

export const Left = Template.bind({});
Left.args = {
	ngContent: 'convoy modal content',
	position: 'left',
    size: 'md',
    title: 'Convoy modal title'
} as Partial<ModalComponent>;

export const Center = Template.bind({});
Center.args = {
	ngContent: 'convoy modal content',
	position: 'center',
    size: 'md',
    title: 'Convoy modal title'
} as Partial<ModalComponent>;

export const Full = Template.bind({});
Full.args = {
	ngContent: 'convoy modal content',
	position: 'full',
    size: 'md',
    title: 'Convoy modal title'
} as Partial<ModalComponent>;
