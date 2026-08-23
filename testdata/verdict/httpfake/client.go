package httpfake

type Response struct {
	Status int
}

func WriteOK() Response {
	return Response{Status: 200}
}

func WriteCreated() Response {
	return Response{Status: 201}
}

func WriteNoContent() Response {
	return Response{Status: 204}
}
